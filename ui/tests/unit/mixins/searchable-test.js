/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { alias } from '@ember/object/computed';
import EmberObject, { computed } from '@ember/object';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Searchable from 'nomad-ui/mixins/searchable';

module('Unit | Mixin | Searchable', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    this.subject = function () {
      // eslint-disable-next-line ember/no-new-mixins
      const SearchableObject = EmberObject.extend(Searchable, {
        source: null,
        searchProps: computed(function () {
          return ['id', 'name'];
        }),
        listToSearch: alias('source'),
      });

      this.owner.register('test-container:searchable-object', SearchableObject);
      return this.owner.lookup('test-container:searchable-object');
    };
  });

  test('the searchable mixin does nothing when there is no search term', function (assert) {
    const subject = this.subject();
    subject.set('source', [
      { id: '1', name: 'hello' },
      { id: '2', name: 'world' },
    ]);

    assert.deepEqual(subject.get('listSearched'), subject.get('source'));
  });

  test('the searchable mixin allows for regex search', function (assert) {
    const subject = this.subject();
    subject.set('source', [
      { id: '1', name: 'hello' },
      { id: '2', name: 'world' },
      { id: '3', name: 'oranges' },
    ]);

    subject.set('searchTerm', '.+l+[A-Z]$');
    assert.deepEqual(
      subject.get('listSearched'),
      [
        { id: '1', name: 'hello' },
        { id: '2', name: 'world' },
      ],
      'hello and world matched for regex'
    );
  });

  test('the searchable mixin only searches the declared search props', function (assert) {
    const subject = this.subject();
    subject.set('source', [
      { id: '1', name: 'United States of America', continent: 'North America' },
      { id: '2', name: 'Canada', continent: 'North America' },
      { id: '3', name: 'Mexico', continent: 'North America' },
    ]);

    subject.set('searchTerm', 'America');
    assert.deepEqual(
      subject.get('listSearched'),
      [
        {
          id: '1',
          name: 'United States of America',
          continent: 'North America',
        },
      ],
      'Only USA matched, since continent is not a search prop'
    );
  });

  test('the fuzzy search mode is off by default', function (assert) {
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

  test('the fuzzy search mode can be enabled', function (assert) {
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
      [
        {
          id: '1',
          name: 'United States of America',
          continent: 'North America',
        },
      ],
      'America is matched due to fuzzy matching'
    );
  });

  test('the fuzzy search can include match results', function (assert) {
    const subject = this.subject();
    subject.set('source', [
      EmberObject.create({
        id: '1',
        name: 'United States of America',
        continent: 'North America',
      }),
      EmberObject.create({
        id: '2',
        name: 'Canada',
        continent: 'North America',
      }),
      EmberObject.create({
        id: '3',
        name: 'Mexico',
        continent: 'North America',
      }),
    ]);

    subject.set('fuzzySearchEnabled', true);
    subject.set('includeFuzzySearchMatches', true);
    subject.set('searchTerm', 'Ameerica');
    assert.deepEqual(
      subject
        .get('listSearched')
        .map((object) =>
          object.getProperties('id', 'name', 'continent', 'fuzzySearchMatches')
        ),
      [
        {
          id: '1',
          name: 'United States of America',
          continent: 'North America',
          fuzzySearchMatches: [
            {
              indices: [
                [2, 2],
                [4, 4],
                [9, 9],
                [11, 11],
                [17, 23],
              ],
              value: 'United States of America',
              key: 'name',
            },
          ],
        },
      ],
      'America is matched due to fuzzy matching'
    );
  });

  test('the exact match search mode can be disabled', function (assert) {
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

  test('the regex search mode can be disabled', function (assert) {
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

  test('each search mode has independent search props', function (assert) {
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
      [
        {
          id: '1',
          name: 'United States of America',
          continent: 'North America',
        },
      ],
      'Fuzzy match on one country, but not an exact match on continent'
    );

    subject.set('searchTerm', '^(.a){3}$');
    assert.deepEqual(
      subject.get('listSearched'),
      [],
      'Canada is not matched by the regex because only id is looked at for regex search'
    );
  });

  test('the resetPagination method is a no-op', function (assert) {
    const subject = this.subject();
    assert.strictEqual(
      subject.get('currentPage'),
      undefined,
      'No currentPage value set'
    );
    subject.resetPagination();
    assert.strictEqual(
      subject.get('currentPage'),
      undefined,
      'Still no currentPage value set'
    );
  });
});

module('Unit | Mixin | Searchable (with pagination)', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    this.subject = function () {
      // eslint-disable-next-line ember/no-new-mixins
      const SearchablePaginatedObject = EmberObject.extend(Searchable, {
        source: null,
        searchProps: computed(function () {
          return ['id', 'name'];
        }),
        listToSearch: alias('source'),
        currentPage: 1,
      });

      this.owner.register(
        'test-container:searchable-paginated-object',
        SearchablePaginatedObject
      );
      return this.owner.lookup('test-container:searchable-paginated-object');
    };
  });

  test('the resetPagination method sets the currentPage to 1', function (assert) {
    const subject = this.subject();
    subject.set('currentPage', 5);
    assert.equal(
      subject.get('currentPage'),
      5,
      'Current page is something other than 1'
    );
    subject.resetPagination();
    assert.equal(subject.get('currentPage'), 1, 'Current page gets reset to 1');
  });
});
