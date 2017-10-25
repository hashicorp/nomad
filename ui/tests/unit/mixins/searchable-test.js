import Ember from 'ember';
import { moduleFor, test } from 'ember-qunit';
import Searchable from 'nomad-ui/mixins/searchable';

const { getOwner, computed } = Ember;

moduleFor('mixin:searchable', 'Unit | Mixin | Searchable', {
  subject() {
    const SearchableObject = Ember.Object.extend(Searchable, {
      source: null,
      searchProps: computed(() => ['id', 'name']),
      listToSearch: computed.alias('source'),
    });

    this.register('test-container:searchable-object', SearchableObject);
    return getOwner(this).lookup('test-container:searchable-object');
  },
});

test('the searchable mixin does nothing when there is no search term', function(assert) {
  const subject = this.subject();
  subject.set('source', [{ id: '1', name: 'hello' }, { id: '2', name: 'world' }]);

  assert.deepEqual(subject.get('listSearched'), subject.get('source'));
});

test('the searchable mixin allows for regex search', function(assert) {
  const subject = this.subject();
  subject.set('source', [
    { id: '1', name: 'hello' },
    { id: '2', name: 'world' },
    { id: '3', name: 'oranges' },
  ]);

  subject.set('searchTerm', '.+l+[A-Z]$');
  assert.deepEqual(
    subject.get('listSearched'),
    [{ id: '1', name: 'hello' }, { id: '2', name: 'world' }],
    'hello and world matched for regex'
  );
});

test('the searchable mixin only searches the declared search props', function(assert) {
  const subject = this.subject();
  subject.set('source', [
    { id: '1', name: 'United States of America', continent: 'North America' },
    { id: '2', name: 'Canada', continent: 'North America' },
    { id: '3', name: 'Mexico', continent: 'North America' },
  ]);

  subject.set('searchTerm', 'America');
  assert.deepEqual(
    subject.get('listSearched'),
    [{ id: '1', name: 'United States of America', continent: 'North America' }],
    'Only USA matched, since continent is not a search prop'
  );
});
