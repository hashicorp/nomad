import { alias } from '@ember/object/computed';
import { getOwner } from '@ember/application';
import EmberObject, { computed } from '@ember/object';
import { moduleFor, test } from 'ember-qunit';
import Searchable from 'nomad-ui/mixins/searchable';

moduleFor('mixin:searchable', 'Unit | Mixin | Searchable', {
  subject() {
    const SearchableObject = EmberObject.extend(Searchable, {
      source: null,
      searchProps: computed(() => ['id', 'name']),
      listToSearch: alias('source'),
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

test('the fuzzy search mode is off by default', function(assert) {
  const subject = this.subject();
  subject.set('source', [
    { id: '1', name: 'United States of America', continent: 'North America' },
    { id: '2', name: 'Canada', continent: 'North America' },
    { id: '3', name: 'Mexico', continent: 'North America' },
  ]);

  subject.set('searchTerm', 'Ameerica');
  assert.deepEqual(
    subject.get('listSearched'),
    [],
    'Nothing is matched since America is spelled incorrectly'
  );
});

test('the fuzzy search mode can be enabled', function(assert) {
  const subject = this.subject();
  subject.set('source', [
    { id: '1', name: 'United States of America', continent: 'North America' },
    { id: '2', name: 'Canada', continent: 'North America' },
    { id: '3', name: 'Mexico', continent: 'North America' },
  ]);

  subject.set('fuzzySearchEnabled', true);
  subject.set('searchTerm', 'Ameerica');
  assert.deepEqual(
    subject.get('listSearched'),
    [{ id: '1', name: 'United States of America', continent: 'North America' }],
    'America is matched due to fuzzy matching'
  );
});

test('the exact match search mode can be disabled', function(assert) {
  const subject = this.subject();
  subject.set('source', [
    { id: '1', name: 'United States of America', continent: 'North America' },
    { id: '2', name: 'Canada', continent: 'North America' },
    { id: '3', name: 'Mexico', continent: 'North America' },
  ]);

  subject.set('regexSearchProps', []);
  subject.set('searchTerm', 'Mexico');

  assert.deepEqual(
    subject.get('listSearched'),
    [{ id: '3', name: 'Mexico', continent: 'North America' }],
    'Mexico is matched exactly'
  );

  subject.set('exactMatchEnabled', false);

  assert.deepEqual(
    subject.get('listSearched'),
    [],
    'Nothing is matched now that exactMatch is disabled'
  );
});

test('the regex search mode can be disabled', function(assert) {
  const subject = this.subject();
  subject.set('source', [
    { id: '1', name: 'United States of America', continent: 'North America' },
    { id: '2', name: 'Canada', continent: 'North America' },
    { id: '3', name: 'Mexico', continent: 'North America' },
  ]);

  subject.set('searchTerm', '^.{6}$');
  assert.deepEqual(
    subject.get('listSearched'),
    [
      { id: '2', name: 'Canada', continent: 'North America' },
      { id: '3', name: 'Mexico', continent: 'North America' },
    ],
    'Canada and Mexico meet the regex criteria'
  );

  subject.set('regexEnabled', false);

  assert.deepEqual(
    subject.get('listSearched'),
    [],
    'Nothing is matched now that regex is disabled'
  );
});

test('each search mode has independent search props', function(assert) {
  const subject = this.subject();
  subject.set('source', [
    { id: '1', name: 'United States of America', continent: 'North America' },
    { id: '2', name: 'Canada', continent: 'North America' },
    { id: '3', name: 'Mexico', continent: 'North America' },
  ]);

  subject.set('fuzzySearchEnabled', true);
  subject.set('regexSearchProps', ['id']);
  subject.set('exactMatchSearchProps', ['continent']);
  subject.set('fuzzySearchProps', ['name']);

  subject.set('searchTerm', 'Nor America');
  assert.deepEqual(
    subject.get('listSearched'),
    [],
    'Not an exact match on continent, not a matchAllTokens match on fuzzy, not a regex match on id'
  );

  subject.set('searchTerm', 'America States');
  assert.deepEqual(
    subject.get('listSearched'),
    [{ id: '1', name: 'United States of America', continent: 'North America' }],
    'Fuzzy match on one country, but not an exact match on continent'
  );

  subject.set('searchTerm', '^(.a){3}$');
  assert.deepEqual(
    subject.get('listSearched'),
    [],
    'Canada is not matched by the regex because only id is looked at for regex search'
  );
});
