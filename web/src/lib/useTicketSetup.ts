import { useMutation, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import {
  createCategory,
  createChannel,
  createRole,
  publishPanel,
  saveTicketSettings,
  ticketConfigQuery,
  type Option,
  type TicketConfig,
} from './tickets'

// Estado y acciones de la configuración de tickets, compartidos por la página de
// ajustes y por el tour guiado para que ambos vean siempre lo mismo.
export function useTicketSetup(guildId: string) {
  const { data: config } = useSuspenseQuery(ticketConfigQuery(guildId))
  const queryClient = useQueryClient()
  const queryKey = ticketConfigQuery(guildId).queryKey

  const [staffRoleId, setStaffRoleId] = useState(config.settings?.staffRoleId ?? '')
  const [categoryId, setCategoryId] = useState(config.settings?.categoryId ?? '')
  const [channelId, setChannelId] = useState(config.settings?.panelChannelId ?? '')

  const refresh = () => queryClient.invalidateQueries({ queryKey })

  // Añade lo recién creado a la lista de opciones.
  function addOption(field: 'roles' | 'categories' | 'textChannels', option: Option) {
    queryClient.setQueryData<TicketConfig>(
      queryKey,
      (old) => old && { ...old, [field]: [...old[field], option] },
    )
  }

  const makeRole = useMutation({
    mutationFn: () => createRole(guildId),
    onSuccess: (option) => {
      addOption('roles', option)
      setStaffRoleId(option.id)
    },
  })

  const makeCategory = useMutation({
    mutationFn: () => createCategory(guildId),
    onSuccess: (option) => {
      addOption('categories', option)
      setCategoryId(option.id)
    },
  })

  const makeChannel = useMutation({
    mutationFn: () => createChannel(guildId),
    onSuccess: (option) => {
      addOption('textChannels', option)
      setChannelId(option.id)
    },
  })

  const save = useMutation({
    mutationFn: () => saveTicketSettings(guildId, { categoryId, staffRoleId }),
    onSuccess: refresh,
  })

  const publish = useMutation({
    mutationFn: () => publishPanel(guildId, channelId),
    onSuccess: refresh,
  })

  return {
    config,
    staffRoleId,
    setStaffRoleId,
    categoryId,
    setCategoryId,
    channelId,
    setChannelId,
    makeRole,
    makeCategory,
    makeChannel,
    save,
    publish,
  }
}

export type TicketSetup = ReturnType<typeof useTicketSetup>
