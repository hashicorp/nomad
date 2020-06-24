export default function isSafari() {
  const oldSafariTest = /constructor/i.test(window.HTMLElement);
  const newSafariTest = (function(p) {
    return p.toString() === '[object SafariRemoteNotification]';
  })(!window['safari'] || (typeof window.safari !== 'undefined' && window.safari.pushNotification));
  return oldSafariTest || newSafariTest;
}
