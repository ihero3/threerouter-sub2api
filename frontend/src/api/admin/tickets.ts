import { apiClient } from '../client'
import type {
  AddTicketMessageRequest,
  BasePaginationResponse,
  Ticket,
  TicketCategory,
  TicketMessage,
  TicketStatus,
  UpdateTicketStatusRequest
} from '@/types'

export interface AdminTicketListParams {
  page?: number
  page_size?: number
  status?: TicketStatus | ''
  category?: TicketCategory | ''
  search?: string
}

export async function list(params: AdminTicketListParams = {}): Promise<BasePaginationResponse<Ticket>> {
  const { data } = await apiClient.get<BasePaginationResponse<Ticket>>('/admin/tickets', { params })
  return data
}

export async function getById(id: number): Promise<Ticket> {
  const { data } = await apiClient.get<Ticket>(`/admin/tickets/${id}`)
  return data
}

export async function updateStatus(id: number, request: UpdateTicketStatusRequest): Promise<Ticket> {
  const { data } = await apiClient.put<Ticket>(`/admin/tickets/${id}/status`, request)
  return data
}

export async function addMessage(id: number, request: AddTicketMessageRequest): Promise<TicketMessage> {
  const { data } = await apiClient.post<TicketMessage>(`/admin/tickets/${id}/messages`, request)
  return data
}

const adminTicketsAPI = {
  list,
  getById,
  updateStatus,
  addMessage
}

export default adminTicketsAPI
