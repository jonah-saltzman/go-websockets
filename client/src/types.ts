export type User = {
    id: string
    name: string
}

export type Message = {
    user: User,
    time: string
    body: string
}

export type LoginResponse = {
    token: string
}

export type GetMessagesResponse = {
    page: number
    messages: Message[]
}

export enum HttpErrors {
    BadRequest = 'BadRequest',
    ServerError = 'ServerError',
    Unauthorized = 'Unauthorized',
}

export const HttpErrorMap: {[key: number]: HttpErrors} = {
    400: HttpErrors.BadRequest,
    401: HttpErrors.Unauthorized,
    500: HttpErrors.ServerError
}

export type SocketConnOpts = {
    token: string
    user: string
    setSocket: React.Dispatch<React.SetStateAction<WebSocket | null>>
    setMessages: React.Dispatch<React.SetStateAction<Message[]>>
    setUserId: React.Dispatch<React.SetStateAction<string>>
    onClose: () => void
}

export type WhoAmIMsg = {
    id: string
    name: string
}

export function isWhoAmI(msg: unknown): msg is WhoAmIMsg {
    if (typeof msg !== 'object' || msg == null)
        return false
    if (!('id' in msg) || !('name' in msg))
        return false
    return typeof msg['id'] === 'string' && typeof msg['name'] === 'string'
}

export function isMessage(msg: unknown): msg is Message {
    if (typeof msg !== 'object' || msg == null)
        return false
    if (!('user' in msg) || !('time' in msg) || !('body' in msg))
        return false
    return typeof msg['user'] === 'object' && typeof msg['time'] === 'string' && typeof msg['body'] === 'string'
}