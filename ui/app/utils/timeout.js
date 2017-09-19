import Ember from 'ember';

const { RSVP } = Ember;

// An always failing promise used to race against other promises
export default function timeout(duration) {
  return new RSVP.Promise((resolve, reject) => {
    setTimeout(() => {
      reject(`Timeout of ${duration}ms exceeded`);
    }, duration);
  });
}
