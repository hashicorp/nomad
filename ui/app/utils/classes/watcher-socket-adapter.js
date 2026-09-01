/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// WatcherSocketAdapter is a simple helper class to run a watcher style
// request using a websocket. It will wire up the given websocket to
// send an initial handshake if a token is provided, listen for abort
// events if a signal is provided, and wait for a response. A promise
// response is provided that will be fulfilled once a message has been
// received. After the message has been received, the websocket will
// be closed.

const CLOSE_NORMAL_CLOSURE_CODE = 1000;

export default class WatcherSocketAdapter {
  constructor(socket, token, signal) {
    this.socket = socket;
    this.token = token;
    this.signal = signal;
    this.resolve = null;
    this.reject = null;

    this.promise = new Promise((resolve, reject) => {
      // If a signal is provided check if it is already aborted.
      if (signal) {
        signal.throwIfAborted();
      }
      // Store these to fulfill later.
      this.resolve = resolve;
      this.reject = reject;
    });

    // If a signal was provided, listen for an abort event to
    // bail out of the request.
    if (signal) {
      signal.addEventListener(
        'abort',
        () => {
          this.rejectPromise(signal.reason);
          this.close();
        },
        { once: true },
      );
    }

    // When the socket is open, if we have a token,
    // send the token to authenticate.
    socket.onopen = () => {
      if (this.token) {
        this.sendWsHandshake();
      }
    };

    // When a message is received, convert it into
    // a response, resolve the promise, and close
    // the socket. Format of the message data:
    // {
    //   "headers": object,
    //   "payload": object
    // }
    socket.onmessage = (event) => {
      let obj = JSON.parse(event.data);
      let response = Response.json(obj.payload, {
        headers: obj.headers,
        status: 200,
      });

      this.resolvePromise(response);
      this.close();
    };

    // If an error is encountered we consider it an unrecoverable
    // state and just reject the response promise and close the
    // socket.
    socket.onerror = () => {
      this.rejectPromise(new Error('fatal websocket error', 'WebSocketError'));
      this.close();
    };

    // When the socket is closed, check if the response promise
    // has been resolved, and if not then reject it.
    socket.onclose = (event) => {
      // If the promise has already been resolved, nothing to do.
      if (!this.resolve) {
        return;
      }

      // Check the reason for the closure. If it's a normal closure,
      // then just reject the promise with a connection closed error.
      if (event.code == CLOSE_NORMAL_CLOSURE_CODE) {
        this.rejectPromise(
          new Error('remote connection closed', 'net::ERR_CONNECTION_CLOSED'),
        );
        return;
      }

      // Construct an error response from the close event.
      let response = new Response(event.reason, { status: 500 });
      this.resolvePromise(response);
    };
  }

  // resolvePromise resolves the response promise with
  // the provided response.
  resolvePromise(response) {
    if (this.resolve) {
      this.resolve(response);
    }
    this.resolve = null;
    this.reject = null;
  }

  // rejectPromise rejects the response promise with
  // the provided reason.
  rejectPromise(reason) {
    if (this.reject) {
      this.reject(reason);
    }
    this.resolve = null;
    this.reject = null;
  }

  // promise returns the promise response for this socket.
  response() {
    return this.promise;
  }

  // close closes the socket.
  close() {
    this.socket.close();
  }

  // sendWsHandshake sends the handshake for authentication with the server.
  sendWsHandshake() {
    this.socket.send(
      JSON.stringify({ version: 1, auth_token: this.token || '' }),
    );
  }
}
