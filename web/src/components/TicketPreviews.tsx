import type { ReactNode } from 'react'
import {
  DiscordButton,
  DiscordCategory,
  DiscordChannel,
  DiscordEmbed,
  DiscordFrame,
  DiscordMention,
  DiscordMessage,
} from './discord/Discord'

// Vistas previas de lo que verán en Discord, con los nombres que se van eligiendo.
// Los textos deben coincidir con los que envía el bot (backend/internal/bot/tickets.go).

// Avatar del bot, en public/.
const BOT_AVATAR = '/askord-avatar.png'

function Preview({ children }: { children: ReactNode }) {
  return <div className="preview">{children}</div>
}

// Paso 3: el panel que se publica en el canal elegido.
export function PanelPreview({ channelName }: { channelName?: string }) {
  return (
    <Preview>
      <DiscordFrame channel={{ name: channelName ?? 'soporte' }}>
        <DiscordMessage author="Askord" app avatarUrl={BOT_AVATAR}>
          <DiscordEmbed title="Soporte">
            ¿Necesitas ayuda? Pulsa el botón para abrir un ticket privado con el staff.
          </DiscordEmbed>
          <DiscordButton variant="secondary" emoji="📩">
            Crear ticket
          </DiscordButton>
        </DiscordMessage>
      </DiscordFrame>
    </Preview>
  )
}

// Paso 2: la categoría con los canales privados de cada ticket.
export function CategoryPreview({ categoryName }: { categoryName?: string }) {
  return (
    <Preview>
      <DiscordFrame tone="sidebar">
        <DiscordCategory name={categoryName ?? 'Tickets'}>
          <DiscordChannel name="ticket-0001" locked />
          <DiscordChannel name="ticket-0002" locked />
          <DiscordChannel name="ticket-0003" locked />
        </DiscordCategory>
      </DiscordFrame>
    </Preview>
  )
}

// Paso 1: el mensaje de bienvenida de un ticket, donde se avisa al rol de staff.
export function WelcomePreview({ roleName }: { roleName?: string }) {
  return (
    <Preview>
      <DiscordFrame channel={{ name: 'ticket-0001', locked: true }}>
        <DiscordMessage author="Askord" app avatarUrl={BOT_AVATAR}>
          Hola <DiscordMention>@usuario</DiscordMention>, gracias por abrir un ticket. Cuéntanos tu
          problema y <DiscordMention>@{roleName ?? 'staff'}</DiscordMention> te atenderá en cuanto
          pueda.
          <div>
            <DiscordButton variant="secondary" emoji="🔒">
              Cerrar ticket
            </DiscordButton>
          </div>
        </DiscordMessage>
      </DiscordFrame>
    </Preview>
  )
}
