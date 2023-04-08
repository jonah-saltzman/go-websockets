;(() => {
    // expectingMessage is set to true
    // if the user has just submitted a message
    // and so we should scroll the next message into view when received.
    let expectingMessage = false
    let shared
    function dial(username, token) {
      console.log(`dialing ${username} / ${token}`)
      const conn = new WebSocket(`ws://${location.host}/join?token=${token}&user=${username}`)
      shared = conn
      conn.addEventListener("close", ev => {
        appendLog(`WebSocket Disconnected code: ${ev.code}, reason: ${ev.reason}`, true)
      })
      conn.addEventListener("open", ev => {
        appendLog("Submit a message to get started!")
        console.info("websocket connected")
      })
  
      // This is where we handle messages received.
      conn.addEventListener("message", ev => {
        if (typeof ev.data !== "string") {
          console.error("unexpected message type", typeof ev.data)
          return
        }
        console.log(ev.data)
        const msg = JSON.parse(ev.data)
        console.log(msg)
        const p = appendLog(`${msg.user.name} (${msg.time}): ${msg.body}`)
        if (expectingMessage) {
          p.scrollIntoView()
          expectingMessage = false
        }
      })
    }

    const userForm = document.getElementById("userbtn")
    userForm.addEventListener('click', async (ev) => {
      try {
        ev.preventDefault()
        const username = document.getElementById("user-input").value
        const password = document.getElementById('pw-input').value
        const url = `http://${location.host}/login`
        console.log({url})
        const r = await fetch(url, {
          method: 'POST',
          headers: {"Content-Type": "application/json"},
          body: JSON.stringify({
            user: username,
            password
          })
        })
        const token = await r.text()
        console.log(token)
        dial(username, token)
      } catch(err) {
        console.log(err)
      }
    })
  
    const messageLog = document.getElementById("message-log")
    const publishForm = document.getElementById("publish-form")
    const messageInput = document.getElementById("message-input")
  
    // appendLog appends the passed text to messageLog.
    function appendLog(text, error) {
      const p = document.createElement("p")
      // Adding a timestamp to each message makes the log easier to read.
      p.innerText = text
      if (error) {
        p.style.color = "red"
        p.style.fontStyle = "bold"
      }
      messageLog.append(p)
      return p
    }
  
    // onsubmit publishes the message from the user when the form is submitted.
    publishForm.onsubmit = async ev => {
      ev.preventDefault()
  
      const msg = messageInput.value
      if (msg === "") {
        return
      }
      messageInput.value = ""
  
      expectingMessage = true
      try {
        console.log(`sending ${msg}`)
        shared.send(msg)
      } catch (err) {
        appendLog(`Publish failed: ${err}`, true)
      }
    }
  })()
  