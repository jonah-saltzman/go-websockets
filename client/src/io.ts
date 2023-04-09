import { GetMessagesResponse, HttpErrorMap, SocketConnOpts, isWhoAmI, isMessage } from "./types"

const baseUrl = window.location.host
const SOCKET_ENDPOINT = `ws://${baseUrl}/join`
const LOGIN_ENDPOINT = `http://${baseUrl}/login`
const LOGOUT_ENDPOINT = `http://${baseUrl}/logout`
const HISTORY_ENDPOINT = `http://${baseUrl}/history`

export async function getHistory(page: number, token: string): Promise<GetMessagesResponse> {
    const response = await fetch(`${HISTORY_ENDPOINT}?page=${page}`, {
        method: 'GET',
        headers: {'Authorization': `Bearer ${token}`}
    })
    handleHttpResponse(response)
    const parsed = await response.json()
    if (!parsed.messages)
        parsed.messages = []
    return parsed
}

export async function login(user: string, password: string): Promise<string> {
    const response = await fetch(LOGIN_ENDPOINT, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({user, password})
    })
    handleHttpResponse(response)
    return (await response.json()).token
}

export async function logout(token: string): Promise<void> {
    const response = await fetch(LOGOUT_ENDPOINT, {
        method: 'GET',
        headers: {'Authorization': `Bearer ${token}`}
    })
    handleHttpResponse(response)
}

export function getSocketConnection(opts: SocketConnOpts) {
    const {token, user, setMessages, setSocket, setUserId, setToken} = opts
    const connection: WebSocket = new WebSocket(`${SOCKET_ENDPOINT}?token=${token}`)
    connection.addEventListener('open', ev => {
        setSocket(connection)
        console.log('ws connected')
    })
    connection.addEventListener('close', ev => {
        setMessages([])
        setSocket(null)
        setToken("")
    })
    connection.addEventListener('message', ev => {
        const parsed = JSON.parse(ev.data)
        if (isWhoAmI(parsed)) {
            if (parsed.name !== user) {
                console.error(`Received incorrect name from server: ${parsed.name}`)
                setSocket(null)
            } else {
                setUserId(parsed.id)
            }
        } else if (isMessage(parsed)) {
            setMessages((prev) => [...prev, parsed])
        } else {
            console.error('Unknown message')
            console.error(ev.data)
        }
    })
    connection.addEventListener('error', ev => {
        console.error(ev)
    })
}

function handleHttpResponse(response: Response) {
    if (response.status !== 200) {
        throw new Error(HttpErrorMap[response.status])
    }
}