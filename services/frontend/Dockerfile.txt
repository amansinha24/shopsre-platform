# ── Stage 1: Build React app ───────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files first — Docker caches this layer
COPY package.json package-lock.json ./

# Install dependencies
RUN npm ci --silent

# Copy source code
COPY . .

# Build production React bundle
RUN npm run build

# ── Stage 2: Serve with nginx ──────────────────────────────────
# nginx is a high performance web server
# Serves the React static files
FROM nginx:alpine

# Copy built React files to nginx
COPY --from=builder /app/build /usr/share/nginx/html

# Copy nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:80 || exit 1

CMD ["nginx", "-g", "daemon off;"]