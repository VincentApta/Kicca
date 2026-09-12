// Admin surface barrel — implementations moved to ./admin/* (2026-09-12 split).
// Kept so existing imports (workspace.tsx, admin-users.render.test.tsx) work
// unchanged; new code should import from ./admin/* directly.
export { UsersPage } from './admin/users'
export { TeamsPage } from './admin/teams'
export { FirstProjectDialog, ProjectSettingsPage } from './admin/project-settings'
