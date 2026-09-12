// カードクリックで編集モーダルを開く。非表示トグルボタンのクリックは
// 伝播を止めて誤って開かないようにする。標準API(<dialog>)のみ使用、外部ライブラリなし。
var editModal = document.getElementById('edit-modal');
var editForm = document.getElementById('edit-form');

// 現在のツールバーのフィルタ状態を、各POST操作へ渡す専用キー名("filter_target"/
// "filter_show_untracked")で取得する。edit/newフォーム自身が"target"という
// 別の意味のフィールドを持つため、フィルタと同じキー名にすると値が衝突する。
function filterStateVals() {
  var filterForm = document.getElementById('filter-state');
  var formData = new FormData(filterForm);
  return {
    filter_target: formData.get('target') || '',
    filter_show_untracked: formData.get('show_untracked') || '',
  };
}

function bindCardActions() {
  document.querySelectorAll('.card-actions').forEach(function (actions) {
    actions.addEventListener('click', function (ev) {
      ev.stopPropagation();
    });
  });

  document.querySelectorAll('.card').forEach(function (card) {
    card.addEventListener('click', function () {
      var editPath = '/tasks/' + card.dataset.taskId + '/edit';
      editForm.setAttribute('action', editPath);
      editForm.setAttribute('hx-post', editPath);
      htmx.process(editForm);

      document.getElementById('edit-title').value = card.dataset.title;
      document.getElementById('edit-description').value = card.dataset.description;
      document.getElementById('edit-target').value = card.dataset.target;
      document.getElementById('edit-status').value = card.dataset.status;
      document.getElementById('edit-due-date').value = card.dataset.dueDate;

      document.getElementById('edit-info-id').textContent = '#' + card.dataset.taskId;
      document.getElementById('edit-info-jira').textContent =
        card.dataset.jiraKey ? card.dataset.jiraKey : '個人タスク(JIRA未連携)';
      document.getElementById('edit-info-tracked').textContent =
        card.dataset.tracked === '1' ? '表示中' : '非表示中';
      document.getElementById('edit-info-created').textContent = card.dataset.createdAt || '-';
      document.getElementById('edit-info-closed').textContent = card.dataset.closedAt || '-';
      document.getElementById('edit-info-synced').textContent = card.dataset.lastSyncedAt || '-';

      var messageBox = document.getElementById('edit-info-message');
      if (card.dataset.msgText) {
        messageBox.classList.remove('hidden');
        document.getElementById('edit-info-message-meta').textContent =
          [card.dataset.msgSource, card.dataset.msgChannel, card.dataset.msgAuthor, card.dataset.msgReceivedAt]
            .filter(Boolean).join(' ・ ');
        document.getElementById('edit-info-message-text').textContent = card.dataset.msgText;
      } else {
        messageBox.classList.add('hidden');
      }

      editModal.showModal();
    });
  });
}
bindCardActions();
// #boardがhtmxでswapされた後、新しいカード要素にもイベントを再バインドする。
document.body.addEventListener('htmx:afterSwap', function (ev) {
  if (ev.target && ev.target.id === 'board') {
    bindCardActions();
    bindDragAndDrop();
  }
});

document.getElementById('edit-cancel').addEventListener('click', function () {
  editModal.close();
});

// 背景(backdrop)クリックで閉じる: dialog要素自身がクリックされた場合のみ閉じる
// (中のフォームへのクリックはbubbleしてもtarget !== dialogになるため誤って閉じない)。
editModal.addEventListener('click', function (ev) {
  if (ev.target === editModal) {
    editModal.close();
  }
});

// 新規タスク用モーダル(編集モーダルと同じ開閉パターン)。
var newTaskModal = document.getElementById('new-task-modal');
var newTaskForm = document.getElementById('new-task-form');

document.getElementById('new-task-btn').addEventListener('click', function () {
  newTaskForm.reset();
  newTaskModal.showModal();
});

document.getElementById('new-task-cancel').addEventListener('click', function () {
  newTaskModal.close();
});

newTaskModal.addEventListener('click', function (ev) {
  if (ev.target === newTaskModal) {
    newTaskModal.close();
  }
});

// 編集・新規作成フォームの送信後、バリデーション成功時のみモーダルを閉じる
// (HTTPステータスは常に200で返るため、成功/失敗はレスポンスヘッダー
// X-Toast-Categoryで判定する)。
document.body.addEventListener('htmx:afterRequest', function (ev) {
  var elt = ev.detail.elt;
  if (elt !== editForm && elt !== newTaskForm) {
    return;
  }
  var category = ev.detail.xhr && ev.detail.xhr.getResponseHeader('X-Toast-Category');
  if (category !== 'error') {
    var dialog = elt.closest('dialog');
    if (dialog) {
      dialog.close();
    }
  }
});

// ドラッグ&ドロップでのステータス変更。標準API(HTML5 Drag and Drop)のみ使用、外部ライブラリなし。
function bindDragAndDrop() {
  document.querySelectorAll('.card').forEach(function (card) {
    card.addEventListener('dragstart', function (ev) {
      ev.dataTransfer.setData('text/plain', card.dataset.taskId);
      card.classList.add('opacity-40');
    });
    card.addEventListener('dragend', function () {
      card.classList.remove('opacity-40');
    });
  });
}
bindDragAndDrop();

// 列の高さ(枠)に関係なく、ポインタのx座標がどの列の左右範囲に入っているかだけで
// ドロップ先を決める。これにより、列の枠の下(ページの余白)にドロップしても、
// 横方向にその列の範囲内であれば移動できる。
function columnAtX(x) {
  var columns = document.querySelectorAll('.column');
  for (var i = 0; i < columns.length; i++) {
    var rect = columns[i].getBoundingClientRect();
    if (x >= rect.left && x <= rect.right) {
      return columns[i];
    }
  }
  return null;
}

function clearColumnHighlights() {
  document.querySelectorAll('.column').forEach(function (c) {
    c.classList.remove('ring-2', 'ring-blue-400');
  });
}

document.addEventListener('dragover', function (ev) {
  ev.preventDefault();
  var hovered = columnAtX(ev.clientX);
  clearColumnHighlights();
  if (hovered) {
    hovered.classList.add('ring-2', 'ring-blue-400');
  }
});

document.addEventListener('drop', function (ev) {
  ev.preventDefault();
  clearColumnHighlights();
  var column = columnAtX(ev.clientX);
  var taskId = ev.dataTransfer.getData('text/plain');
  if (!column || !taskId) { return; }
  htmx.ajax('POST', '/tasks/' + taskId + '/move', {
    target: '#board',
    swap: 'outerHTML',
    values: Object.assign({
      status: column.dataset.status,
    }, filterStateVals()),
  });
});

// タブを開きっぱなしにしても他の変更が反映されるよう、定期的に自動リロードする。
// ただしモーダル編集中に中身が消えないよう、モーダルが開いている間はスキップする。
var AUTO_REFRESH_INTERVAL_MS = 45000;
setInterval(function () {
  if (editModal.open || newTaskModal.open) { return; }
  var filterForm = document.getElementById('filter-state');
  htmx.ajax('GET', '/', { target: '#board', swap: 'outerHTML', select: '#board', values: Object.fromEntries(new FormData(filterForm)) });
}, AUTO_REFRESH_INTERVAL_MS);
