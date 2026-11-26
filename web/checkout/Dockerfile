FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /repo
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY gen/ts gen/ts
COPY web/checkout web/checkout
RUN pnpm install --frozen-lockfile --filter @pay/checkout...
RUN pnpm --filter @pay/checkout build

FROM node:24-alpine
WORKDIR /app
COPY --from=build /repo/web/checkout/build ./build
COPY --from=build /repo/web/checkout/package.json ./
ENV NODE_ENV=production PORT=3000
EXPOSE 3000
CMD ["node", "build"]
