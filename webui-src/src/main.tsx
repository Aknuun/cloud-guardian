import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './app.css'
import './deepseek.css'
import './utilities.css'
import './panel.css'
import { initMode, ThemeProvider } from './components/theme'
import App from './App'

initMode()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </StrictMode>,
)
