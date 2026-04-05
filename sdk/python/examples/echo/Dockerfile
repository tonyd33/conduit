# Build stage
FROM python:3.12-slim AS builder

WORKDIR /build

# Copy SDK
COPY ../.. /build/sdk
WORKDIR /build/sdk/examples/echo

# Install dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Runtime stage
FROM python:3.12-slim

WORKDIR /app

# Copy application
COPY --from=builder /build/sdk/examples/echo/main.py .
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages

CMD ["python", "main.py"]
