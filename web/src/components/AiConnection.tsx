import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { aiQuery, deleteAi, saveAi, testAi, type AiConnection as Connection } from '../lib/ai'
import { toast } from '../lib/toast'
import { Button } from './ui/Button'
import { Dialog } from './ui/Dialog'
import { FormField } from './ui/FormField'
import { Input } from './ui/Input'
import './ai-connection.css'

type Props = { guildId: string; connection: Connection }

// Conexión con el LLM: sin ninguna guardada, el formulario; con una, un resumen con un botón
// para editarla en un diálogo y otro para eliminarla.
export function AiConnection({ guildId, connection }: Props) {
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)
  const [removeOpen, setRemoveOpen] = useState(false)

  const remove = useMutation({
    mutationFn: () => deleteAi(guildId),
    onSuccess: () => {
      queryClient.setQueryData(aiQuery(guildId).queryKey, {
        ...connection,
        baseUrl: '',
        model: '',
        enabled: false,
        hasKey: false,
      })
      setRemoveOpen(false)
      toast('Conexión eliminada')
    },
  })

  if (!connection.hasKey) {
    return <AiForm guildId={guildId} connection={connection} />
  }

  return (
    <>
      <div className="ai-table-wrap">
        <table className="ai-table">
          <thead>
            <tr>
              <th>URL de la API</th>
              <th>Modelo</th>
              <th>
                <span className="visually-hidden">Acciones</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>{connection.baseUrl}</td>
              <td>{connection.model}</td>
              <td className="ai-table-actions">
                <Button variant="outline" onClick={() => setEditOpen(true)}>
                  Editar
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <section className="danger-zone">
        <h2>Eliminar conexión</h2>
        <p>Borra la URL, el modelo y la API key guardada de este servidor.</p>
        <Button variant="danger" onClick={() => setRemoveOpen(true)}>
          Eliminar conexión
        </Button>
      </section>

      <Dialog open={editOpen} onOpenChange={setEditOpen} title="Editar conexión con el LLM">
        <AiForm
          guildId={guildId}
          connection={connection}
          onSaved={() => setEditOpen(false)}
          onCancel={() => setEditOpen(false)}
        />
      </Dialog>

      <Dialog open={removeOpen} onOpenChange={setRemoveOpen} title="Eliminar la conexión con el LLM">
        <p>Se borrará la API key guardada y el asistente dejará de poder usar este proveedor.</p>
        {remove.isError && <small className="error">{remove.error.message}</small>}
        <div className="tour-actions">
          <Button variant="outline" onClick={() => setRemoveOpen(false)} disabled={remove.isPending}>
            Cancelar
          </Button>
          <Button variant="danger" onClick={() => remove.mutate()} loading={remove.isPending}>
            {remove.isPending ? 'Eliminando' : 'Eliminar'}
          </Button>
        </div>
      </Dialog>
    </>
  )
}

type FormProps = {
  guildId: string
  connection: Connection
  // Presentes cuando el formulario va en un diálogo.
  onSaved?: () => void
  onCancel?: () => void
}

// URL, modelo y API key, con prueba de conexión.
function AiForm({ guildId, connection, onSaved, onCancel }: FormProps) {
  const queryClient = useQueryClient()
  const [baseUrl, setBaseUrl] = useState(connection.baseUrl)
  const [model, setModel] = useState(connection.model)
  const [apiKey, setApiKey] = useState('')

  const input = { baseUrl, model, apiKey, enabled: connection.enabled }
  const ready = baseUrl.trim() !== '' && model.trim() !== '' && (apiKey !== '' || connection.hasKey)

  const test = useMutation({ mutationFn: () => testAi(guildId, input) })
  const save = useMutation({
    mutationFn: () => saveAi(guildId, input),
    onSuccess: (saved) => {
      queryClient.setQueryData(aiQuery(guildId).queryKey, saved)
      setApiKey('')
      toast('Conexión guardada')
      onSaved?.()
    },
  })

  // Cualquier cambio invalida el resultado de la última prueba.
  function edit(set: (v: string) => void) {
    return (e: { target: { value: string } }) => {
      set(e.target.value)
      test.reset()
      save.reset()
    }
  }

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    save.mutate()
  }

  return (
    <form className={onCancel ? 'form ai-dialog-form' : 'form'} onSubmit={onSubmit}>
      <FormField
        label="URL de la API"
        hint="Cualquier API compatible con OpenAI, sin /chat/completions. Por ejemplo https://api.openai.com/v1 o https://openrouter.ai/api/v1."
      >
        {({ id }) => (
          <Input
            id={id}
            type="url"
            value={baseUrl}
            onChange={edit(setBaseUrl)}
            placeholder="https://api.openai.com/v1"
            autoComplete="off"
            spellCheck={false}
          />
        )}
      </FormField>

      <FormField label="Modelo" hint="El nombre exacto del modelo, tal como lo llama tu proveedor.">
        {({ id }) => (
          <Input
            id={id}
            value={model}
            onChange={edit(setModel)}
            placeholder="gpt-4o-mini"
            autoComplete="off"
            spellCheck={false}
          />
        )}
      </FormField>

      <FormField
        label="API key"
        hint={
          connection.hasKey
            ? 'Hay una clave guardada. Déjalo vacío para conservarla o escribe otra para cambiarla.'
            : 'Se guarda cifrada y no se vuelve a mostrar.'
        }
      >
        {({ id }) => (
          <Input
            id={id}
            type="password"
            value={apiKey}
            onChange={edit(setApiKey)}
            placeholder={connection.hasKey ? '••••••••••••' : 'sk-…'}
            autoComplete="new-password"
            spellCheck={false}
          />
        )}
      </FormField>

      <div className="form-actions">
        <Button type="submit" disabled={!ready} loading={save.isPending}>
          {save.isPending ? 'Guardando' : 'Guardar'}
        </Button>
        <Button
          variant="outline"
          disabled={!ready}
          loading={test.isPending}
          onClick={() => test.mutate()}
        >
          {test.isPending ? 'Probando' : 'Probar conexión'}
        </Button>
        {onCancel && (
          <Button variant="outline" onClick={onCancel} disabled={save.isPending}>
            Cancelar
          </Button>
        )}
      </div>

      {test.data?.ok && <p className="success">La conexión funciona.</p>}
      {test.data && !test.data.ok && <p className="error">{test.data.message}</p>}
      {test.isError && <p className="error">{test.error.message}</p>}
      {save.isError && <p className="error">{save.error.message}</p>}
    </form>
  )
}
