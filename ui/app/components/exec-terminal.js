import Component from '@ember/component';
import { Terminal } from 'xterm';
import base64js from 'base64-js';
import { TextDecoderLite, TextEncoderLite } from 'text-encoder-lite';

export default Component.extend({
  didInsertElement() {
    this.terminal = new Terminal();
    this.terminal.open(this.element.querySelector('.terminal'));

    this.terminal.onData(e => {
      this.socket.send(JSON.stringify({ stdin: { data: encodeString(e) } }));
    });

    // FIXME this is a hack to provide access in an integration test 🧐
    window.xterm = this.terminal;

    this.socket.onmessage = e => {
      const json = JSON.parse(e.data);
      this.terminal.write(decodeString(json.stdout.data));
    };
  },
});

function encodeString(string) {
  var encoded = new TextEncoderLite('utf-8').encode(string);
  return base64js.fromByteArray(encoded);
}

function decodeString(b64String) {
  var uint8array = base64js.toByteArray(b64String);
  return new TextDecoderLite('utf-8').decode(uint8array);
}
