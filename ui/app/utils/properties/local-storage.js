import { computed } from '@ember/object';

// An Ember.Computed property that persists set values in localStorage
// and will attempt to get its initial value from localStorage before
// falling back to a default.
//
// ex. showTutorial: localStorageProperty('nomadTutorial', true),
export default function localStorageProperty(localStorageKey, defaultValue) {
  return computed({
    get() {
      const persistedValue = window.localStorage.getItem(localStorageKey);
      return persistedValue ? JSON.parse(persistedValue) : defaultValue;
    },
    set(key, value) {
      window.localStorage.setItem(localStorageKey, JSON.stringify(value));
      return value;
    },
  });
}
