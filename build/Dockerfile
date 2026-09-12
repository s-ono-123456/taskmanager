FROM python:3.13-slim

WORKDIR /app

RUN pip install --no-cache-dir flask

# ビルドコンテキストはリポジトリルート（compose参照）。
COPY db.py seed.py app.py ./
COPY templates ./templates

EXPOSE 8090

CMD ["python", "app.py"]
