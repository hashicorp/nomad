export default class XHRToken {
  capture(xhr) {
    this._xhr = xhr;
  }

  abort() {
    if (this._xhr) {
      this._xhr.abort();
    }
  }
}
