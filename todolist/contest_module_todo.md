# Contest Module TODO List

> Generated on 2026-03-07. Focused on unfinished logic in `services/contest_service/internal/logic`.

## Core Lifecycle
- **Publish contest** (`services/contest_service/internal/logic/publishlogic.go`)
  - Implement publish state transition and validation.
  - Add idempotency + state checks as per contest lifecycle rules.
- **Close contest** (`services/contest_service/internal/logic/closelogic.go`)
  - Implement close/end transition with validation.

## Registration & Participation
- **Register** (`services/contest_service/internal/logic/registerlogic.go`)
  - Validate eligibility, create participant record, cache invalidation.
- **Approve** (`services/contest_service/internal/logic/approvelogic.go`)
  - Admin approval workflow + status transition.
- **Quit** (`services/contest_service/internal/logic/quitlogic.go`)
  - Participant quit logic, cache invalidation.
- **Participants list** (`services/contest_service/internal/logic/participantslogic.go`)
  - Paginated list of participants.

## Teams
- **Create team** (`services/contest_service/internal/logic/teamcreatelogic.go`)
- **Join team** (`services/contest_service/internal/logic/teamjoinlogic.go`)
- **Leave team** (`services/contest_service/internal/logic/teamleavelogic.go`)
- **Team list** (`services/contest_service/internal/logic/teamlistlogic.go`)

## Problems (Contest Problemset)
- **Add problem** (`services/contest_service/internal/logic/problemaddlogic.go`)
- **Update problem** (`services/contest_service/internal/logic/problemupdatelogic.go`)
- **Remove problem** (`services/contest_service/internal/logic/problemremovelogic.go`)
- **Problem list** (`services/contest_service/internal/logic/problemlistlogic.go`)

## Leaderboard & Results
- **Leaderboard** (`services/contest_service/internal/logic/leaderboardlogic.go`)
- **Frozen leaderboard** (`services/contest_service/internal/logic/leaderboardfrozenlogic.go`)
- **Leaderboard replay** (`services/contest_service/internal/logic/leaderboardreplaylogic.go`)
- **My result** (`services/contest_service/internal/logic/myresultlogic.go`)
- **Member result** (`services/contest_service/internal/logic/memberresultlogic.go`)

## Hacks
- **Create hack** (`services/contest_service/internal/logic/hackcreatelogic.go`)
- **Get hack** (`services/contest_service/internal/logic/hackgetlogic.go`)

## Announcements
- **Create announcement** (`services/contest_service/internal/logic/announcementcreatelogic.go`)
- **Announcement list** (`services/contest_service/internal/logic/announcementlistlogic.go`)

## Notes
- All items above currently contain the goctl scaffold TODO placeholder.
- Ensure each module follows cache rules, error codes, logx usage, and context timeouts.
