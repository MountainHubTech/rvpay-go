This is a [Next.js](https://nextjs.org) project bootstrapped with [`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app).

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/app/building-your-application/deploying) for more details.

## Dockerization Workflow

This application is being prepared for production Docker deployment through a sequence of numbered Cline agents. The workflow is controlled by the project-control files in this repository:

- `.clinerules.md` — rules all agents must follow
- `.clineignore.md` — areas agents should avoid reading unless directly required
- `.project-context.md` — stable facts, decisions, and unknowns
- `.project-checkpoint.md` — chronological checkpoint ledger with reading map
- `.clinecheck.md` — verification checklist for the sequence
- `.project-next-steps` — AWS deployment guide (populated only after the sequence completes)

AWS deployment steps are documented only after Agent 06 completes the Dockerization sequence.

## Docker / AWS-Readiness Status

The Dockerization sequence is complete for local production capability. The production image builds and the container starts successfully.

- **Build:** multi-stage `node:22-alpine` build (`npm ci` from the committed lockfile → `next build` → minimal standalone runtime). `next.config.ts` uses `output: "standalone"`.
- **Runtime:** minimal standalone server, listens on port **3000**, runs as non-root `nodejs` (uid/gid 1001), with `NODE_ENV=production` baked in.
- **Image hygiene:** `.dockerignore` excludes env files and secret material; verified no `.env` / `.pem` / `.key` in the image.
- **Environment:** the application uses **zero** environment variables; no `.env` is required at build or runtime.
- **Workflow:** `make build`, `make run`, `make stop`, `make verify` cover the local Docker workflow. `make tag` / `make ecr-login` / `make ecr-push` are a thin AWS ECR push interface (require `AWS_REGION`, `AWS_ACCOUNT_ID`, `AWS_ECR_REPO`; no credentials are committed).
- **AWS readiness:** the image can be built reproducibly, tagged, pushed to ECR, and run with runtime values, and it can sit behind a load balancer on port 3000. No AWS infrastructure is created here. Deployment steps and open architecture decisions are in `.project-next-steps.md`.
