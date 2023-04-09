import React, { useState, useEffect, useRef } from 'react'
import { login, getHistory, getSocketConnection, logout } from './io'
import './App.css'
import { Message } from './types'
import { MessageBox } from './Message'

const App = () => {

    const [user, setUser] = useState('')
    const [password, setPassword] = useState('')
    const [token, setToken] = useState('')
    const [userId, setUserId] = useState('')
    const [messages, setMessages] = useState<Message[]>([])
    const [page, setPage] = useState(-1)
    const [socket, setSocket] = useState<WebSocket | null>(null)
    const [isLoading, setIsLoading] = useState(false)
    const [message, setMessage] = useState('')
    const [shouldLogin, setShouldLogin] = useState(false)

    const messagesEndRef = useRef<HTMLDivElement>(null)

    const handleLogin = async (user: string, password: string) => {
        if (!user || !password) return
        try {
            const token = await login(user, password)
            setToken(token)
            getSocketConnection({ token, user, setMessages, setSocket, setUserId, setToken })
        } catch (err) {
            alert(err)
        }
    }

    const handleLogout = async (token: string) => {
        if (!token) return
        try {
            await logout(token)
        } catch (err) {
            alert(err)
        }
    }

    const handleGetHistory = async () => {
        setIsLoading(true)
        try {
            if (page === -2) return
            const response = await getHistory(page, token)
            setMessages((prev) => [...prev, ...response.messages])
            setPage(response.page === 0 ? -2 : response.page - 1)
        } catch (err) {
            alert(err)
        } finally {
            setIsLoading(false)
        }
    }

    const handleSendMessage = (msg: string) => {
        if (!socket) return
        socket.send(msg)
    }

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }

    const handleScroll = (e: React.UIEvent<HTMLDivElement, UIEvent>) => {
        const { scrollTop } = e.target as HTMLElement
        if (scrollTop === 0 && !isLoading) {
            getHistory(page, token)
        }
    }

    useEffect(() => {
        if (shouldLogin) {
            setShouldLogin(false)
            handleLogin(user, password)
        }
    }, [shouldLogin, user, password])

    useEffect(() => {
        if (token) {
            handleGetHistory()
        }
    }, [token])

    useEffect(() => {
        scrollToBottom()
    }, [messages])

    return (
        <div className="container">
            {token && (
                <div className="logout-wrapper">
                    <button className="logout-button" onClick={() => handleLogout(token)}>Logout</button>
                </div>
            )}
            {!token && (
                <form id='login-form' className='form' onSubmit={e => {
                    e.preventDefault()
                    //handleLogin(user, password)
                }}>
                    <input
                        type="text"
                        placeholder="Username"
                        value={user}
                        onChange={(e) => setUser(e.target.value)}
                    />
                    <input
                        type="password"
                        placeholder="Password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                    />
                    <button onClick={() => setShouldLogin(true)}>Login</button>
                </form>
            )}
            {token && (
                <div className="chat" onScroll={handleScroll}>
                    {messages.map((message, index) => (
                        <MessageBox message={message} index={index} me={userId} />
                    ))}
                    <div ref={messagesEndRef} />
                </div>
            )}
            {token && (
                <div className="message-input">
                    <input
                        type="text"
                        placeholder="Type your message"
                        value={message}
                        onChange={(e) => setMessage(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                handleSendMessage(message);
                                setMessage('');
                            }
                        }}
                    />
                </div>
            )}
        </div>
    )
}

export default App
