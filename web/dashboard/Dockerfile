FROM node:24-alpine AS build
ARG VITE_API_URL=/api
ENV VITE_API_URL=$VITE_API_URL
RUN corepack enable
WORKDIR /repo
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY gen/ts gen/ts
COPY web/dashboard web/dashboard
RUN pnpm install --frozen-lockfile --filter @pay/dashboard...
RUN pnpm --filter @pay/dashboard build

FROM nginx:1.29-alpine
COPY web/dashboard/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /repo/web/dashboard/dist /usr/share/nginx/html
