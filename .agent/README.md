# Bolt Agent Team — Shared State

This directory is the communication bus for all AI agents on the bolt project.

## Directory Structure

```
.agent/
  status/       — each agent writes their current status here
  backlog/      — product backlog and task queue
  messages/     — inter-agent messages (agent writes here to reach another)
  reports/      — heartbeat reports and summaries
```

## Status File Format

Each agent maintains `.agent/status/<agent>.md`:

```
# <Agent> Status
**Last Updated**: YYYY-MM-DD HH:MM
**Status**: Idle | Working | Blocked | Done
**Current Task**: <description or "None">
**Blockers**: <description or "None">
**Recent Output**: <1-3 line summary of last action>
**Next Steps**: <what this agent will do next>
```

## Communication Protocol

- To send a message to an agent, append to `.agent/messages/<recipient>.md`
- Always include sender name and timestamp
- Manager reads all status files during heartbeat and coordinates
- Agents should update their status file after every meaningful action

## Agents

| Skill       | Role                        |
|-------------|-----------------------------|
| /manager    | Team orchestrator           |
| /po         | Product Owner               |
| /scrum      | Scrum Master                |
| /backend    | Go Backend Developer        |
| /qa         | QA Engineer                 |
| /networking | Networking / Protocol Expert|
| /security   | Security Expert             |
| /devops     | DevOps / CI-CD              |
| /architect  | System Architect            |
| /researcher | Research (Perplexity)       |
| /marketing  | Marketing / Launch          |
