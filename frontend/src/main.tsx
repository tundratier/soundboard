import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Admin } from './admin/Admin'
import { BoardView } from './board/BoardView'
import './styles.css'

// One build, two views: /admin (localhost only) and / (board for everyone on the network).
const isAdmin = location.pathname === '/admin' || location.pathname.startsWith('/admin/')

createRoot(document.getElementById('root')!).render(<StrictMode>{isAdmin ? <Admin /> : <BoardView />}</StrictMode>)
