--- 
on:
  schedule: daily

permissions:
  contents: read
  issues: read
  pull-requests: read

safe-outputs:
  create-issue:
    title-prefix: "[repo-status] "
    labels:
      - report

tools:
  github:
--- 

Daily Repository Status Report
Create a daily report for maintainers.
Include:
Recent activity (issues, PRs, commits)
Key highlights and risks
Recommended next steps
Keep the report concise and link to relevant issues and pull requests.
