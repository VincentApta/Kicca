// Self-service profile (#49): the current (non-admin) user edits their name
// and password. Email is read-only — login identity, admin-only path.
import { useEffect, useState, type FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ApiError, api } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { validateProfile, type FieldErrors } from '@/lib/admin'
import { useToast } from '@/lib/toast'

export function ProfilePage() {
  const { state, updateUser } = useAuth()
  const me = state.phase === 'authenticated' ? state.user : null
  const toast = useToast()

  const [name, setName] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (me) {
      setName(me.name)
      setErrors({})
    }
  }, [me?.id])

  async function saveProfile(e: FormEvent) {
    e.preventDefault()
    if (!me) return
    const v = validateProfile({ name, currentPassword, newPassword, confirmPassword })
    setErrors(v)
    if (Object.keys(v).length > 0) return
    setBusy(true)
    try {
      const updated = await api.patchMe({
        name: name.trim(),
        ...(newPassword ? { current_password: currentPassword, new_password: newPassword } : {}),
      })
      updateUser(updated)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      toast('Profile saved')
    } catch (err) {
      if (err instanceof ApiError && err.status === 422) {
        setErrors({ form: err.message }) // e.g. wrong current password
      } else {
        setErrors({ form: 'Could not save profile.' })
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex-1 overflow-auto p-6">
      <h1 className="font-heading text-2xl font-semibold text-foreground">Profile</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Your account — name and password. Email is your login identity; ask an admin to change it.
      </p>

      <form className="card-neu mt-6 flex max-w-xl flex-col gap-4 p-4" onSubmit={saveProfile}>
        <div className="flex flex-col gap-2">
          <Label htmlFor="profile-name">Name</Label>
          <Input
            id="profile-name"
            className="inset-neu"
            aria-invalid={!!errors.name}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          {errors.name && (
            <p role="alert" className="text-xs text-destructive">
              {errors.name}
            </p>
          )}
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="profile-email">Email</Label>
          <Input id="profile-email" className="inset-neu font-mono" value={me?.email ?? ''} disabled />
        </div>

        <div className="mt-2 border-t border-border pt-4">
          <h2 className="font-heading text-base font-semibold text-foreground">Change password</h2>
          <div className="mt-3 grid gap-4 sm:grid-cols-3">
            <div className="flex flex-col gap-2">
              <Label htmlFor="profile-current" className="text-xs text-muted-foreground">
                Current
              </Label>
              <Input
                id="profile-current"
                type="password"
                className="inset-neu"
                autoComplete="current-password"
                aria-invalid={!!errors.currentPassword}
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
              />
              {errors.currentPassword && (
                <p role="alert" className="text-xs text-destructive">
                  {errors.currentPassword}
                </p>
              )}
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="profile-new" className="text-xs text-muted-foreground">
                New
              </Label>
              <Input
                id="profile-new"
                type="password"
                className="inset-neu"
                autoComplete="new-password"
                aria-invalid={!!errors.newPassword}
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
              {errors.newPassword && (
                <p role="alert" className="text-xs text-destructive">
                  {errors.newPassword}
                </p>
              )}
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="profile-confirm" className="text-xs text-muted-foreground">
                Confirm
              </Label>
              <Input
                id="profile-confirm"
                type="password"
                className="inset-neu"
                autoComplete="new-password"
                aria-invalid={!!errors.confirmPassword}
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
              {errors.confirmPassword && (
                <p role="alert" className="text-xs text-destructive">
                  {errors.confirmPassword}
                </p>
              )}
            </div>
          </div>
        </div>

        {errors.form && (
          <p role="alert" className="text-sm text-destructive">
            {errors.form}
          </p>
        )}
        <div className="flex justify-end">
          <Button type="submit" disabled={busy}>
            {busy ? 'Saving…' : 'Save changes'}
          </Button>
        </div>
      </form>
    </div>
  )
}
