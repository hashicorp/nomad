import { modifier } from 'ember-modifier';

export default modifier(function windowResize(element, [handler]) {
  const boundHandler = ev => handler(element, ev);
  window.addEventListener('resize', boundHandler);

  return () => {
    window.removeEventListener('resize', boundHandler);
  };
});
