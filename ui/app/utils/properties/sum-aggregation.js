import Ember from 'ember';

const { computed } = Ember;

// An Ember.Computed property for summating all properties from a
// set of objects.
//
// ex. list: [ { foo: 1 }, { foo: 3 } ]
//     sum: sumAggregationProperty('list', 'foo') // 4
export default function sumAggregationProperty(listKey, propKey) {
  return computed(`${listKey}.@each.${propKey}`, function() {
    return this.get(listKey).mapBy(propKey).reduce((sum, count) => sum + count, 0);
  });
}
