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
      document.getElementById('edit-priority').value = card.dataset.priority;

      document.getElementById('edit-info-id').textContent = '#' + card.dataset.taskId;
      document.getElementById('edit-info-cycle').textContent =
        card.dataset.cycle ? '今週（週開始: ' + card.dataset.cycle + '）' : 'バックログ';
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

// クローズ要求一覧モーダル(論点C3)。承認/却下を続けて処理できるよう、
// 編集・新規作成モーダルと違って送信後も自動では閉じない(htmx:afterRequestの対象外)。
var closeRequestsModal = document.getElementById('close-requests-modal');

document.getElementById('close-requests-btn').addEventListener('click', function () {
  closeRequestsModal.showModal();
});

document.getElementById('close-requests-cancel').addEventListener('click', function () {
  closeRequestsModal.close();
});

closeRequestsModal.addEventListener('click', function (ev) {
  if (ev.target === closeRequestsModal) {
    closeRequestsModal.close();
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

// ドロップ先は「スイムレーン行(y座標)」→「ステータス列(x座標)」の2段階で決定する。
// まずY座標で.lane-row(今週/バックログ)を特定し、次にその行の内側に限定して、
// 列の高さ(枠)に関係なくx座標がどの列の左右範囲に入っているかだけで列を決める
// (これにより、列の枠の下(行内の余白)にドロップしても、横方向にその列の範囲内で
// あれば移動できる、という既存の寛容な挙動を維持する)。

// スイムレーン行の当たり判定に余裕を持たせるための追加マージン(px)。行間のgap-6(24px)
// より小さい値にして、2行の拡張ヒット領域が重ならないようにする(誤操作防止)。
var LANE_HIT_MARGIN_PX = 10;

function laneRowAtY(y) {
  var rows = document.querySelectorAll('.lane-row');
  for (var i = 0; i < rows.length; i++) {
    var rect = rows[i].getBoundingClientRect();
    if (y >= rect.top - LANE_HIT_MARGIN_PX && y <= rect.bottom + LANE_HIT_MARGIN_PX) {
      return rows[i];
    }
  }
  return null;
}

function columnAtX(container, x) {
  var columns = container.querySelectorAll('.column');
  for (var i = 0; i < columns.length; i++) {
    var rect = columns[i].getBoundingClientRect();
    if (x >= rect.left && x <= rect.right) {
      return columns[i];
    }
  }
  return null;
}

function dropTargetAt(x, y) {
  var laneRow = laneRowAtY(y);
  if (!laneRow) { return null; }
  return columnAtX(laneRow, x);
}

function clearColumnHighlights() {
  document.querySelectorAll('.column').forEach(function (c) {
    c.classList.remove('ring-2', 'ring-blue-400');
  });
}

document.addEventListener('dragover', function (ev) {
  ev.preventDefault();
  var hovered = dropTargetAt(ev.clientX, ev.clientY);
  clearColumnHighlights();
  if (hovered) {
    hovered.classList.add('ring-2', 'ring-blue-400');
  }
});

document.addEventListener('drop', function (ev) {
  ev.preventDefault();
  clearColumnHighlights();
  var column = dropTargetAt(ev.clientX, ev.clientY);
  var taskId = ev.dataTransfer.getData('text/plain');
  if (!column || !taskId) { return; }
  htmx.ajax('POST', '/tasks/' + taskId + '/move', {
    target: '#board',
    swap: 'outerHTML',
    values: Object.assign({
      status: column.dataset.status,
      cycle: column.dataset.lane,
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
