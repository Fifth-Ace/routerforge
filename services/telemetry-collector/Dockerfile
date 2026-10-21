FROM python:3.13-slim
ENV PYTHONUNBUFFERED=1 RF_PORT=8000 RF_DB_PATH=/data/telemetry.sqlite
WORKDIR /app
COPY collector.py /app/collector.py
RUN useradd --system --uid 10001 --home /data rfcollector && mkdir -p /data && chown -R 10001:10001 /data
USER 10001:10001
EXPOSE 8000
CMD ["python", "-B", "collector.py"]
