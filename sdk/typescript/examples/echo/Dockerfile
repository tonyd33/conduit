# Build stage
FROM node:20-slim AS builder

WORKDIR /build

# Copy SDK
COPY ../.. /build/sdk
WORKDIR /build/sdk/examples/echo

# Install dependencies
RUN npm install

# Build
RUN npm run build

# Runtime stage
FROM node:20-slim

WORKDIR /app

# Copy built application
COPY --from=builder /build/sdk/examples/echo/dist ./dist
COPY --from=builder /build/sdk/examples/echo/node_modules ./node_modules
COPY --from=builder /build/sdk/examples/echo/package.json ./

CMD ["npm", "start"]
