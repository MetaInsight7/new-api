const ADMIN_ONLY_SECTIONS = new Set<string>(['users', 'codex-limits'])

export function isDashboardSectionAllowed(
  section: string,
  isAdmin = false
): boolean {
  return !ADMIN_ONLY_SECTIONS.has(section) || isAdmin
}
