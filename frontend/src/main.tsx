import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Admin } from './admin/Admin'
import { BoardView } from './board/BoardView'
import './styles.css'

// Ein Build, zwei Ansichten: /admin (nur localhost) und / (Board für alle im Netz).
const isAdmin = location.pathname === '/admin' || location.pathname.startsWith('/admin/')

createRoot(document.getElementById('root')!).render(<StrictMode>{isAdmin ? <Admin /> : <BoardView />}</StrictMode>)
