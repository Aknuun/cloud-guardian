import { SignOut } from '@phosphor-icons/react'
import { haptic, setToken, TG } from '../lib'
import { Button } from './ui'

export function LogoutButton() {
  function logout() {
    haptic('light')
    setToken('')
    try {
      sessionStorage.clear()
    } catch {
      /* ignore */
    }
    try {
      localStorage.setItem('cg_signed_out', '1')
    } catch {
      /* ignore */
    }
    window.dispatchEvent(new Event('cg:signed-out'))
    try {
      if (TG && TG.close) TG.close()
    } catch {
      /* ignore */
    }
  }

  return (
    <Button
      variant="ghost"
      shape="square"
      icon={SignOut}
      aria-label="خروج کامل"
      title="خروج کامل"
      onClick={logout}
    />
  )
}
