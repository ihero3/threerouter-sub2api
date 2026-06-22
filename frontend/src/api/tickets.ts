import { apiClient } from './client'
import type {
  AddTicketMessageRequest,
  BasePaginationResponse,
  CreateTicketRequest,
  Ticket,
  TicketMessage
} from '@/types'

export async function create(request: CreateTicketRequest, authenticated: boolean): Promise<Ticket> {
  const path = authenticated ? '/tickets' : '/public/tickets'
  const { data } = await apiClient.post<Ticket>(path, request)
  return data
}

export async function listMine(page: number = 1, pageSize: number = 20): Promise<BasePaginationResponse<Ticket>> {
  const { data } = await apiClient.get<BasePaginationResponse<Ticket>>('/tickets/my', {
    params: { page, page_size: pageSize }
  })
  return data
}

export async function getMine(id: number): Promise<Ticket> {
  const { data } = await apiClient.get<Ticket>(`/tickets/${id}`)
  return data
}

export async function addMessage(id: number, request: AddTicketMessageRequest): Promise<TicketMessage> {
  const { data } = await apiClient.post<TicketMessage>(`/tickets/${id}/messages`, request)
  return data
}

const ticketsAPI = {
  create,
  listMine,
  getMine,
  addMessage
}

export default ticketsAPI
