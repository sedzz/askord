# Retención y borrado de transcripciones

> Estado: **implementado** (migración 00012, `internal/cleanup`, `store/retention.go`). Quedan pendientes la selección múltiple en la lista y avisar del plazo en los términos o la política de privacidad antes de producción.
>
> Diferencias respecto al diseño original: `tickets.content_purged_at` marca los tickets cuyo contenido ya se borró (a mano o por caducidad) y el bot rechaza transcribirlos; el reparto entre copias de la API se hace con `FOR UPDATE SKIP LOCKED` por lote, sin candado asesor; el plazo es `store.TranscriptRetention`; y solo se crea el índice `tickets_retention_idx` (el de la lista queda para cuando haga falta).

## Decisión

Askord **no guarda el contenido de un ticket más de 90 días**. Pasado ese plazo se borra solo. Además, la gente que administra un servidor puede borrar una transcripción a mano antes de que caduque.

Motivos:

- **Privacidad**: se guarda solo lo necesario y solo el tiempo necesario. El contenido de un ticket (lo que se dijo y las respuestas de los formularios) es lo que contiene datos personales.
- **Escala**: el volumen de datos tiene un techo, porque solo crece la actividad de los últimos tres meses. Evita tener que rediseñar el almacenamiento si hay millones de tickets.
- **Ahorro**: menos datos que guardar, copiar y consultar.

## Qué se borra y qué se queda

| Se borra | Se queda |
|---|---|
| Los mensajes de la transcripción (`ticket_messages`) | El registro del ticket: número, tipo, fechas, estado |
| Las respuestas del formulario (`tickets.answers`) | El identificador de Discord de quien lo abrió (`opener_id`) |

El registro del ticket se conserva para que las estadísticas del dashboard (tickets abiertos, tiempo medio de cierre, tickets por tipo) no cambien. La transcripción borrada deja de aparecer en la lista de Transcripciones.

Más adelante, cuando las estadísticas diarias estén resumidas en su propia tabla, se podrán borrar también los registros antiguos. No hace falta ahora.

## Cuándo caduca

**90 días desde que se cierra el ticket** (`closed_at`). Un ticket abierto no tiene transcripción, así que no caduca. El plazo es una constante en el código; empezamos con el mismo para todos los servidores y no será configurable al principio.

## Dos formas de borrar

1. **Manual**, desde el detalle de una transcripción: botón rojo **Eliminar transcripción** con confirmación ("no se puede deshacer"). Sirve para atender a quien pide que se borre su conversación antes del plazo. Solo para quien administra el servidor.
   - Más adelante: selección múltiple en la lista, y "eliminar las que coinciden con el filtro".
2. **Automática**, a los 90 días, mediante una tarea diaria.

## La tarea diaria: en el backend, no en la base de datos

Decidido: **la programa y la lanza el backend (Go)**; la base de datos solo ejecuta el borrado.

- `pg_cron` necesita una extensión que no viene en la imagen de Postgres que usamos ni en todos los servicios gestionados. Dependeríamos de la infraestructura y el entorno local no la tendría.
- En el backend el código se revisa, se prueba y se versiona con el resto, y los registros salen por el log normal.
- El mismo "limpiador" servirá para más cosas: **borrar las sesiones de login caducadas** (hoy se acumulan, `DeleteExpiredSessions` existe pero nadie la llama) y, en el futuro, reintentar webhooks fallidos.

### Cómo funcionaría

- Se lanza **al arrancar** la API y luego **cada pocas horas**. Si la API estuvo apagada un día, al volver recupera lo atrasado: el borrado es repetible.
- El borrado va **por lotes** (unos cientos de tickets por vuelta) para no bloquear la base de datos, y repite hasta no encontrar más.
- Si hubiera **varias copias** de la API, un candado de Postgres (`pg_try_advisory_lock`) hace que solo una ejecute la tarea.
- Se escribe en el log cuántos tickets se limpiaron en cada ejecución.

### Esbozo de la consulta

```sql
-- Tickets cerrados hace más de 90 días que aún conservan contenido, en un lote.
WITH expired AS (
    SELECT id FROM tickets
    WHERE status = 'closed'
      AND closed_at < now() - interval '90 days'
      AND (answers <> '[]'::jsonb
           OR EXISTS (SELECT 1 FROM ticket_messages m WHERE m.ticket_id = tickets.id))
    LIMIT 500
), gone AS (
    DELETE FROM ticket_messages WHERE ticket_id IN (SELECT id FROM expired)
)
UPDATE tickets SET answers = '[]'::jsonb WHERE id IN (SELECT id FROM expired);
```

Índices que ayudan (una migración):

```sql
CREATE INDEX tickets_closed_at_idx ON tickets (closed_at) WHERE status = 'closed';
CREATE INDEX tickets_guild_created_idx ON tickets (guild_id, created_at DESC);
```

## Cambios en la interfaz y el bot

- **Página de Transcripciones**: una nota fija, "Las transcripciones se eliminan automáticamente a los 90 días", para que se pueda exportar antes.
- **Detalle de una transcripción**: cuándo se borrará ("Se eliminará el 12 de enero").
- **Botón Transcribir (Discord)**: si el ticket se cerró hace más de 90 días, responde que **ya ha caducado** y no lo guarda. Si no, un canal que aún existe volvería a generar lo que se acaba de borrar.
- **El aviso al borrar a mano** debe mencionar lo mismo: si el canal del ticket sigue existiendo en Discord, el staff podría volver a pulsar Transcribir.

## Casos a tener en cuenta

- **Antes de desplegarlo en producción**, avisar del plazo en los términos o la política de privacidad.
- La primera ejecución borra lo que ya supere los 90 días; con datos de desarrollo no importa, pero en producción conviene saberlo.
- Las **exportaciones** no se ven afectadas: el archivo descargado es del usuario.
- El `opener_id` (ID de Discord) se queda mientras exista el registro del ticket; es un identificador, no el contenido.

## Orden de trabajo previsto

1. Borrado manual desde el detalle (API `DELETE /api/guilds/{id}/transcripts/{ticketID}`, botón y diálogo de confirmación) y la migración con los índices.
2. Tarea diaria de borrado a los 90 días, con candado y registro, más la limpieza de sesiones.
3. Avisos en la interfaz y rechazo de transcribir tickets caducados.

## Si algún día el volumen crece mucho

Con la retención de 90 días probablemente no haga falta, pero por orden de menos a más esfuerzo:

1. Guardar ya calculados en `tickets` el recuento de mensajes, quién abrió el ticket y la primera frase, para que la lista no recorra los mensajes.
2. Paginación por cursor en lugar de por desplazamiento, y una tabla de participantes con índice para el filtro por usuario.
3. Resumir las estadísticas del dashboard en una tabla diaria.
4. Exportaciones grandes como tarea en segundo plano, con enlace de descarga.
5. Particionar `ticket_messages` por mes (el borrado por retención pasa a ser instantáneo) y, para la búsqueda por contenido, el texto completo de Postgres o un buscador aparte.
