// Thin wrapper around window.Telegram.WebApp so the app also runs in a normal browser (dev).
import { onBeforeUnmount, onMounted, watch } from 'vue'

export const tg = window.Telegram?.WebApp ?? null

// True when opened inside Telegram (initData is only present there).
export const inTelegram = Boolean(tg && tg.initData)

export function initTelegram() {
  if (!tg) return
  tg.ready()
  tg.expand()
  applyTheme()
  tg.onEvent?.('themeChanged', applyTheme)
}

function applyTheme() {
  document.documentElement.dataset.scheme = tg?.colorScheme || 'light'
}

export function haptic(kind = 'light') {
  try {
    if (kind === 'success' || kind === 'error' || kind === 'warning') tg?.HapticFeedback?.notificationOccurred(kind)
    else tg?.HapticFeedback?.impactOccurred(kind)
  } catch {
    // Haptics are optional.
  }
}

export function openTelegramLink(url) {
  if (tg?.openTelegramLink && url.startsWith('https://t.me/')) tg.openTelegramLink(url)
  else window.open(url, '_blank', 'noopener')
}

export async function confirmDialog(message) {
  if (tg?.showConfirm && inTelegram) {
    return new Promise((resolve) => tg.showConfirm(message, (ok) => resolve(ok)))
  }
  return window.confirm(message)
}

export function alertDialog(message) {
  if (tg?.showAlert && inTelegram) tg.showAlert(message)
  else window.alert(message)
}

// Shows Telegram's BackButton while the component is mounted (no-op outside Telegram).
export function useBackButton(onBack) {
  if (!tg?.BackButton) return
  onMounted(() => {
    tg.BackButton.onClick(onBack)
    tg.BackButton.show()
  })
  onBeforeUnmount(() => {
    tg.BackButton.offClick(onBack)
    tg.BackButton.hide()
  })
}

// Drives Telegram's MainButton from reactive state. Outside Telegram, the view renders its own
// button using the returned `native` flag.
export function useMainButton(state, onClick) {
  const native = Boolean(tg?.MainButton && inTelegram)
  if (!native) return { native }
  const sync = () => {
    const s = state()
    if (!s || !s.visible) {
      tg.MainButton.hide()
      return
    }
    tg.MainButton.setText(s.text)
    if (s.disabled) tg.MainButton.disable()
    else tg.MainButton.enable()
    if (s.loading) tg.MainButton.showProgress()
    else tg.MainButton.hideProgress()
    tg.MainButton.show()
  }
  onMounted(() => {
    tg.MainButton.onClick(onClick)
    sync()
  })
  watch(state, sync, { deep: true })
  onBeforeUnmount(() => {
    tg.MainButton.offClick(onClick)
    tg.MainButton.hide()
  })
  return { native }
}
