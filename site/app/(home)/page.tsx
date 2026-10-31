import { Why } from '@/components/landing/drift';
import { Machinery } from '@/components/landing/bento';
import { HowItWorks } from '@/components/landing/how';
import { GuidesSensors } from '@/components/landing/loop';
import { Proof } from '@/components/landing/proof';
import { HerdrHero } from '@/components/landing/capture';
import { ArrowRight, Button, Container, GitHubIcon, Reveal } from '@/components/landing/ui';
import { githubUrl } from '@/lib/shared';

function Hero() {
  return (
    <section className="noise relative overflow-hidden pt-16 pb-20 sm:pt-24 sm:pb-28">
      {/* texture: a faded grid and one warm glow behind the terminal */}
      <div className="bg-grid mask-fade-b pointer-events-none absolute inset-0 opacity-70" aria-hidden />
      <div
        className="pointer-events-none absolute left-1/2 top-[52%] h-[420px] w-[min(900px,100%)] -translate-x-1/2 rounded-full bg-ember opacity-[0.09] blur-[110px] dark:opacity-[0.12]"
        aria-hidden
      />
      <Container className="relative">
        <Reveal className="mx-auto max-w-3xl text-center">
          <a
            href="#how"
            className="group mx-auto inline-flex items-center gap-2 rounded-full border border-line-strong bg-ink-1/70 py-1 pr-3 pl-1 text-xs text-fg-muted backdrop-blur transition-colors hover:text-fg"
          >
            <span className="rounded-full bg-ember-soft px-2 py-0.5 font-mono text-[11px] text-ember-ink">CLI</span>
            <span>
              <span className="hidden sm:inline">Sessions, an inbox and a worker </span>
              <span className="sm:hidden">Sessions and an inbox </span>for Claude Code
            </span>
            <ArrowRight className="size-3.5" />
          </a>
          <h1 className="mt-7 text-display font-semibold text-balance text-fg">
            There is no universal harness. <span className="text-ember-ink">Build yours.</span>
          </h1>
          <p className="mx-auto mt-6 max-w-2xl text-lead text-pretty text-fg-muted">
            ness is a CLI that turns Claude Code into a team of sessions in your repo. You talk to one agent. It opens the
            rest, each in its own worktree, branch and terminal tab. Your phone pings only when you are needed.
          </p>
          <div className="mt-9 flex flex-col items-stretch justify-center gap-3 sm:flex-row sm:items-center">
            <Button href="/docs/start/quickstart">
              Get started <ArrowRight />
            </Button>
            <Button href="/docs" variant="ghost">
              Read the docs
            </Button>
          </div>
          <p className="mt-5 font-mono text-[11.5px] text-fg-subtle">Linux · Claude Code · Codex (experimental)</p>
        </Reveal>

        <div className="mx-auto mt-14 max-w-5xl sm:mt-16">
          <HerdrHero />
        </div>
      </Container>
    </section>
  );
}

function GetStarted() {
  return (
    <section className="py-20 sm:py-28">
      <Container>
        <Reveal>
          <div className="relative overflow-hidden rounded-2xl border border-line bg-ink-1 px-6 py-14 text-center sm:px-12 sm:py-20">
            <div className="bg-dots mask-fade-b pointer-events-none absolute inset-0 opacity-80" aria-hidden />
            <div
              className="pointer-events-none absolute -bottom-40 left-1/2 h-80 w-[min(640px,100%)] -translate-x-1/2 rounded-full bg-ember opacity-[0.12] blur-[90px]"
              aria-hidden
            />
            <div className="relative">
              <h2 className="text-h2 font-semibold text-balance text-fg">Build yours.</h2>
              <p className="mx-auto mt-4 max-w-md text-lead text-pretty text-fg-muted">
                Run ness init in your repo and open your first session. The quickstart walks through it.
              </p>
              <div className="mt-8 flex flex-col items-stretch justify-center gap-3 sm:flex-row sm:items-center">
                <Button href="/docs/start/quickstart">
                  Get started <ArrowRight />
                </Button>
                <Button href={githubUrl} variant="ghost">
                  <GitHubIcon /> Star on GitHub
                </Button>
              </div>
            </div>
          </div>
        </Reveal>
      </Container>
    </section>
  );
}

export default function Home() {
  return (
    <main className="flex-1">
      <Hero />
      <HowItWorks />
      <GuidesSensors />
      <Machinery />
      <Why />
      <Proof />
      <GetStarted />
    </main>
  );
}
