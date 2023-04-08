import React from 'react'
import { Message } from './types'

type MessageProps = {
    message: Message
    me: string
    index: number
}
export const MessageBox = (props: MessageProps): JSX.Element => {
    const { index, message, me } = props
    const timestamp = new Date(message.time).toLocaleTimeString()
    return (
        <div
            key={index}
            className={`chat-message-wrapper ${message.user.id === me ? 'chat-message-right' : 'chat-message-left'}`}
        >
            <div className="chat-message">
                <div className="message-content">{message.body}</div>
            </div>
            {message.user.id !== me &&
                (<div className="message-info">
                    <span className="sender">{message.user.name}</span> - <span className="timestamp">{timestamp}</span>
                </div>)}
        </div>
    )
}