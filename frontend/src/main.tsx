import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Admin } from './admin/Admin'
import { BoardView } from './board/BoardView'
import { BoardsOverview } from './board/BoardsOverview'
import './styles.css'

// One build, several views, chosen by path:
//   /admin          admin UI (localhost only)
//   /boards/<id>    one board
//   /boards         overview of all boards (also the fallback; the server redirects / to it)
function route() {
  const path = location.pathname.replace(/\/+$/, '')
  if (path === '/admin' || path.startsWith('/admin/')) return <Admin />
  const m = path.match(/^\/boards\/([^/]+)$/)
  if (m) return <BoardView boardId={decodeURIComponent(m[1])} />
  return <BoardsOverview />
}

createRoot(document.getElementById('root')!).render(<StrictMode>{route()}</StrictMode>)
