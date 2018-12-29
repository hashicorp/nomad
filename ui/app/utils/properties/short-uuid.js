import Ember from 'ember';

const { computed } = Ember;

// An Ember.Computed property for taking the first segment
// of a uuid.
//
// ex. id: 123456-7890-abcd-efghijk
//     short: shortUUIDProperty('id') // 123456
export default function shortUUIDProperty(uuidKey) {
  return computed(uuidKey, function() {
    return this.get(uuidKey).split('-')[0];
  });
}
