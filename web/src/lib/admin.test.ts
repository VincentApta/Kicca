import { describe, expect, it } from 'vitest'

import {
  canChangeMemberRole,
  canRemoveMember,
  soleProjectAdmin,
  toMemberPayload,
  validateLabel,
  validateProjectGeneral,
  validateProjectGithub,
  validateTeamName,
  validateUserCreate,
  validateUserEdit,
} from './admin'

describe('validateUserCreate', () => {
  it('passes a complete valid form', () => {
    expect(
      validateUserCreate({ email: 'ada@kica.dev', name: 'Ada', password: 'hunter2hunter2' }),
    ).toEqual({})
  })

  it('requires every field', () => {
    const errors = validateUserCreate({ email: '', name: '', password: '' })
    expect(errors.email).toBeTruthy()
    expect(errors.name).toBeTruthy()
    expect(errors.password).toBeTruthy()
  })

  it('rejects malformed email', () => {
    expect(validateUserCreate({ email: 'nope', name: 'A', password: 'longenough' }).email).toBeTruthy()
    expect(validateUserCreate({ email: 'a@b', name: 'A', password: 'longenough' }).email).toBeTruthy()
  })

  it('rejects short passwords', () => {
    expect(
      validateUserCreate({ email: 'a@b.dev', name: 'A', password: 'short' }).password,
    ).toBeTruthy()
  })

  it('trims name and email before judging', () => {
    expect(validateUserCreate({ email: '  a@b.dev  ', name: '   ', password: 'longenough' }).name).toBeTruthy()
    expect(validateUserCreate({ email: '  a@b.dev  ', name: ' A ', password: 'longenough' })).toEqual({})
  })
})

describe('validateUserEdit', () => {
  it('allows blank password (keep current)', () => {
    expect(validateUserEdit('Ada', '')).toEqual({})
  })

  it('requires name and enforces password minimum when set', () => {
    expect(validateUserEdit('  ', '').name).toBeTruthy()
    expect(validateUserEdit('Ada', 'short').password).toBeTruthy()
    expect(validateUserEdit('Ada', 'longenough')).toEqual({})
  })
})

describe('validateTeamName', () => {
  it('requires non-blank name', () => {
    expect(validateTeamName('Platform')).toEqual({})
    expect(validateTeamName('   ').name).toBeTruthy()
  })
})

describe('validateProjectGeneral', () => {
  it('accepts a valid name + key', () => {
    expect(validateProjectGeneral({ name: 'Kica', key: 'KIC' })).toEqual({})
    expect(validateProjectGeneral({ name: 'Kica', key: 'K1' })).toEqual({})
    expect(validateProjectGeneral({ name: 'Kica', key: 'ABCDE12345' })).toEqual({})
  })

  it('rejects bad keys per contract regex', () => {
    const bad = (key: string) => validateProjectGeneral({ name: 'Kica', key })
    expect(bad('kic').key).toBeTruthy() // lowercase start
    expect(bad('K').key).toBeTruthy() // too short
    expect(bad('KIC12345678').key).toBeTruthy() // 11 chars
    expect(bad('KIC-1').key).toBeTruthy() // dash
    expect(bad('KIC_1').key).toBeTruthy() // underscore
    expect(bad('').key).toBeTruthy()
  })

  it('requires name', () => {
    expect(validateProjectGeneral({ name: ' ', key: 'KIC' }).name).toBeTruthy()
  })
})

describe('validateLabel', () => {
  it('accepts name with or without color', () => {
    expect(validateLabel({ name: 'bug', color: '#0ea5e9' })).toEqual({})
    expect(validateLabel({ name: 'bug', color: '' })).toEqual({})
  })

  it('requires name and hex color', () => {
    expect(validateLabel({ name: '  ', color: '#0ea5e9' }).name).toBeTruthy()
    expect(validateLabel({ name: 'bug', color: 'red' }).color).toBeTruthy()
    expect(validateLabel({ name: 'bug', color: '#fff' }).color).toBeTruthy()
    expect(validateLabel({ name: 'bug', color: '0ea5e9' }).color).toBeTruthy()
  })
})

describe('soleProjectAdmin guard', () => {
  const member = (user_id: string, role: string) => ({ user_id, role })

  it('returns null with no admins or multiple admins', () => {
    expect(soleProjectAdmin([])).toBeNull()
    expect(
      soleProjectAdmin([member('a', 'member'), member('b', 'member')]),
    ).toBeNull()
    expect(
      soleProjectAdmin([member('a', 'project_admin'), member('b', 'project_admin')]),
    ).toBeNull()
  })

  it('returns the sole admin id when exactly one remains', () => {
    expect(
      soleProjectAdmin([member('a', 'project_admin'), member('b', 'member')]),
    ).toBe('a')
  })

  it('blocks role change and removal only for the sole admin', () => {
    const members = [member('a', 'project_admin'), member('b', 'member')]
    expect(canChangeMemberRole(members, 'a')).toBe(false)
    expect(canRemoveMember(members, 'a')).toBe(false)
    expect(canChangeMemberRole(members, 'b')).toBe(true)
    expect(canRemoveMember(members, 'b')).toBe(true)
  })

  it('unblocks everyone once a second admin exists', () => {
    const members = [
      member('a', 'project_admin'),
      member('b', 'project_admin'),
      member('c', 'member'),
    ]
    expect(canChangeMemberRole(members, 'a')).toBe(true)
    expect(canRemoveMember(members, 'b')).toBe(true)
  })
})

describe('toMemberPayload', () => {
  it('keeps valid roles and normalizes stray admin display role', () => {
    expect(
      toMemberPayload([
        { user_id: 'a', role: 'project_admin' },
        { user_id: 'b', role: 'member' },
        { user_id: 'c', role: 'admin' },
      ]),
    ).toEqual([
      { user_id: 'a', role: 'project_admin' },
      { user_id: 'b', role: 'member' },
      { user_id: 'c', role: 'member' },
    ])
  })
})

describe('validateProjectGithub', () => {
  it('passes owner/name repo with a token', () => {
    expect(validateProjectGithub({ repo: 'acme/app', token: 'ghp_x', hasStoredToken: false })).toEqual({})
  })

  it('allows a blank token when one is already stored', () => {
    expect(validateProjectGithub({ repo: 'acme/app', token: '', hasStoredToken: true })).toEqual({})
  })

  it('requires a token on first save', () => {
    expect(validateProjectGithub({ repo: 'acme/app', token: '', hasStoredToken: false }).token).toBeTruthy()
  })

  it('rejects repos without a single owner/name slash split', () => {
    for (const repo of ['', 'acme', 'acme/app/extra', 'a b/c']) {
      expect(validateProjectGithub({ repo, token: 'ghp_x', hasStoredToken: false }).repo).toBeTruthy()
    }
  })
})
