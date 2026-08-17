# Frontend image for the starrank.dev droplet. Build from web/:
#   docker buildx build --platform linux/amd64 -t ghta-web:latest --load \
#     --build-arg API_URL=https://api.starrank.dev \
#     --build-arg NEXT_PUBLIC_SITE_URL=https://starrank.dev \
#     --build-arg NEXT_PUBLIC_GA_ID=<ga id> \
#     web/
#
# NEXT_PUBLIC_* values are inlined into the client bundle at build time, and
# the SSG pages fetch real data during `next build`, so API_URL must point at
# the public API here (the compose-internal http://api:3000 only exists at
# runtime, where it takes over via the environment).

# ---- deps ----
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# ---- build ----
FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG API_URL
ARG NEXT_PUBLIC_SITE_URL
ARG NEXT_PUBLIC_GA_ID
ENV API_URL=$API_URL \
    NEXT_PUBLIC_SITE_URL=$NEXT_PUBLIC_SITE_URL \
    NEXT_PUBLIC_GA_ID=$NEXT_PUBLIC_GA_ID \
    NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ---- run ----
FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production \
    PORT=3000 \
    HOSTNAME=0.0.0.0 \
    NEXT_TELEMETRY_DISABLED=1
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
COPY --from=build --chown=node:node /app/public ./public
# Blog posts are read from disk at request time (src/lib/blog.ts uses
# process.cwd()/content/blog), so the tracer can't see them — ship explicitly.
COPY --from=build --chown=node:node /app/content ./content
USER node
EXPOSE 3000
CMD ["node", "server.js"]
