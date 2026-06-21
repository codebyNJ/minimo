# Product Requirements — Plain Language Version

*Written for anyone, not just programmers. If a sentence needs a CS degree to understand, it's wrong — flag it.*

## The Problem

If you use more than one AI coding assistant — say Claude Code on one project and Codex or Cursor on another — you have no single place to see what each one is doing. You have to open each tool separately to check: is it still working? How long has it been running? Is it about to run out of "memory" (context) and need to be restarted?

There's also a more painful problem: if one assistant hits its limit or gets rate-limited mid-task, you currently lose the thread of what it was doing. You have to re-explain everything to a different assistant from scratch.

## The Idea

Build a small program called `ctx` that runs in your terminal (the black-and-white command window, not a website or app). It shows a live dashboard — like a car's dashboard — with one panel per AI assistant you have installed. Each panel shows:

- Is it currently active or sitting idle?
- How long has the current session been running?
- Roughly how "full" its memory is (so you know when it's about to need a reset)

Think of it like a TV remote that shows the status of every streaming device in your house on one screen, instead of checking each TV individually.

## What Makes This Different

Every AI assistant already keeps its own private notes on disk (a session log) — `ctx` doesn't change how the assistants work; it just **reads their notes** and displays them nicely. It never sends your data anywhere and never calls any AI itself — it's a plain monitoring tool, like a thermometer, not a brain.

Later, `ctx` will also let you **copy the gist of what one assistant was working on and hand it to another assistant** — so if Claude Code runs out of room, you can move the baton to Codex without starting over.

## Who This Is For

People who already use one or more AI coding assistants daily and juggle them across projects. If you only ever use a single assistant in a single terminal tab, this tool adds little value to you today — though the "hand off context" feature may still help.

## What We Are NOT Building

- Not a chatbot, not an AI assistant itself
- Not a cloud service — everything stays on your own computer
- Not a way to control or send commands to the other assistants — read-only monitoring first, with a hand-off feature later
- Not a code editor or IDE plugin

## Success Looks Like

1. You run one command and see, at a glance, which of your AI assistants are active right now and how close each is to its memory limit.
2. You never have to alt-tab between four terminal windows just to check "is it still working."
3. When one assistant gets cut off, handing its context to another takes one command instead of a manual copy-paste essay.

## Open Questions (need a human decision, not a guess)

See [Decisions](Decisions.md) for the running log of things we deliberately chose not to assume.
