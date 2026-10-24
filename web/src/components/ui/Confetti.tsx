import { useState } from 'react'
import './confetti.css'

// Colores pastel, en la línea de la paleta de la app.
const COLORS = ['#ffc5d3', '#ffffff', '#ffe3a3', '#c9b6ff', '#aef0d1', '#a8d8ff']
const PIECES = 70

type Piece = { style: Record<string, string> }

function makePiece(): Piece {
  const between = (min: number, max: number) => min + Math.random() * (max - min)
  const side = between(5, 10)
  return {
    style: {
      '--x': `${between(0, 100)}%`,
      '--dx': `${between(-120, 120)}px`,
      '--w': `${side}px`,
      '--h': `${Math.random() < 0.5 ? side : side * between(1.6, 2.4)}px`,
      '--r': Math.random() < 0.35 ? '50%' : '2px',
      '--rot': `${between(360, 1080)}deg`,
      '--c': COLORS[Math.floor(Math.random() * COLORS.length)],
      '--delay': `${between(0, 0.9)}s`,
      '--d': `${between(2.4, 4)}s`,
    },
  }
}

// Lluvia de confeti a pantalla completa, de una sola vez. No bloquea los clics y,
// para quien prefiere menos movimiento, no se muestra.
export function Confetti() {
  const [pieces] = useState(() => Array.from({ length: PIECES }, makePiece))

  return (
    <div className="ui-confetti" aria-hidden="true">
      {pieces.map((piece, i) => (
        <span key={i} className="ui-confetti-piece" style={piece.style} />
      ))}
    </div>
  )
}
