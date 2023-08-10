/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { findAll, find, render } from '@ember/test-helpers';
import { module, skip, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | list pagination', function (hooks) {
  setupRenderingTest(hooks);

  const defaults = {
    source: [],
    size: 25,
    page: 1,
    spread: 2,
  };

  const list100 = Array(100)
    .fill(null)
    .map((_, i) => i);

  test('the source property', async function (assert) {
    assert.expect(36);

    this.set('source', list100);
    await render(hbs`
      <ListPagination @source={{source}} as |p|>
        <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
        <p.first><span class="first">first</span></p.first>
        <p.prev><span class="prev">prev</span></p.prev>
        {{#each p.pageLinks as |link|}}
          <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
        {{/each}}
        <p.next><span class="next">next</span></p.next>
        <p.last><span class="last">last</span></p.last>

        {{#each p.list as |item|}}
          <div class="item">{{item}}</div>
        {{/each}}
      </ListPagination>
    `);

    assert.notOk(
      findAll('.first').length,
      'On the first page, there is no first link'
    );
    assert.notOk(
      findAll('.prev').length,
      'On the first page, there is no prev link'
    );
    await componentA11yAudit(this.element, assert);

    assert.equal(
      findAll('.link').length,
      defaults.spread + 1,
      'Pages links spread to the right by the spread amount'
    );

    for (var pageNumber = 1; pageNumber <= defaults.spread + 1; pageNumber++) {
      assert.ok(
        findAll(`.link.page-${pageNumber}`).length,
        `Page link includes ${pageNumber}`
      );
    }

    assert.ok(
      findAll('.next').length,
      'While not on the last page, there is a next link'
    );
    assert.ok(
      findAll('.last').length,
      'While not on the last page, there is a last link'
    );
    await componentA11yAudit(this.element, assert);

    assert.equal(
      findAll('.item').length,
      defaults.size,
      `Only ${defaults.size} (the default) number of items are rendered`
    );

    for (var item = 0; item < defaults.size; item++) {
      assert.equal(
        findAll('.item')[item].textContent,
        item,
        'Rendered items are in the current page'
      );
    }
  });

  test('the size property', async function (assert) {
    this.setProperties({
      size: 5,
      source: list100,
    });
    await render(hbs`
      <ListPagination @source={{source}} @size={{size}} as |p|>
        <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
      </ListPagination>
    `);

    const totalPages = Math.ceil(this.source.length / this.size);
    assert.equal(
      find('.page-info').textContent,
      `1 of ${totalPages}`,
      `${totalPages} total pages`
    );
  });

  test('the spread property', async function (assert) {
    assert.expect(12);

    this.setProperties({
      source: list100,
      spread: 1,
      size: 10,
      currentPage: 5,
    });

    await render(hbs`
      <ListPagination @source={{source}} @spread={{spread}} @size={{size}} @page={{currentPage}} as |p|>
        {{#each p.pageLinks as |link|}}
          <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
        {{/each}}
      </ListPagination>
    `);

    testSpread.call(this, assert);
    this.set('spread', 4);
    testSpread.call(this, assert);
  });

  test('page property', async function (assert) {
    assert.expect(10);

    this.setProperties({
      source: list100,
      size: 5,
      currentPage: 5,
    });

    await render(hbs`
      <ListPagination @source={{source}} @size={{size}} @page={{currentPage}} as |p|>
        {{#each p.list as |item|}}
          <div class="item">{{item}}</div>
        {{/each}}
      </ListPagination>
    `);

    testItems.call(this, assert);
    this.set('currentPage', 2);
    testItems.call(this, assert);
  });

  // Ember doesn't support query params (or controllers or routes) in integration tests,
  // so links can only be tested in acceptance tests.
  // Leaving this test here for posterity.
  skip('pagination links link with query params', function () {});

  test('there are no pagination links when source is less than page size', async function (assert) {
    this.set('source', list100.slice(0, 10));
    await render(hbs`
      <ListPagination @source={{source}} as |p|>
        <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
        <p.first><span class="first">first</span></p.first>
        <p.prev><span class="prev">prev</span></p.prev>
        {{#each p.pageLinks as |link|}}
          <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
        {{/each}}
        <p.next><span class="next">next</span></p.next>
        <p.last><span class="last">last</span></p.last>

        {{#each p.list as |item|}}
          <div class="item">{{item}}</div>
        {{/each}}
      </ListPagination>
    `);

    assert.notOk(findAll('.first').length, 'No first link');
    assert.notOk(findAll('.prev').length, 'No prev link');
    assert.notOk(findAll('.next').length, 'No next link');
    assert.notOk(findAll('.last').length, 'No last link');

    assert.equal(find('.page-info').textContent, '1 of 1', 'Only one page');
    assert.equal(
      findAll('.item').length,
      this.get('source.length'),
      'Number of items equals length of source'
    );
  });

  // when there is less pages than the total spread amount
  test('when there is less pages than the total spread amount', async function (assert) {
    assert.expect(9);

    this.setProperties({
      source: list100,
      spread: 4,
      size: 20,
      page: 3,
    });

    const totalPages = Math.ceil(this.get('source.length') / this.size);

    await render(hbs`
      <ListPagination @source={{source}} @page={{page}} @spread={{spread}} @size={{size}} as |p|>
        <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
        <p.first><span class="first">first</span></p.first>
        <p.prev><span class="prev">prev</span></p.prev>
        {{#each p.pageLinks as |link|}}
          <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
        {{/each}}
        <p.next><span class="next">next</span></p.next>
        <p.last><span class="last">last</span></p.last>
      </ListPagination>
    `);

    assert.ok(findAll('.first').length, 'First page still exists');
    assert.ok(findAll('.prev').length, 'Prev page still exists');
    assert.ok(findAll('.next').length, 'Next page still exists');
    assert.ok(findAll('.last').length, 'Last page still exists');
    assert.equal(
      findAll('.link').length,
      totalPages,
      'Every page gets a page link'
    );
    for (var pageNumber = 1; pageNumber < totalPages; pageNumber++) {
      assert.ok(
        findAll(`.link.page-${pageNumber}`).length,
        `Page link for ${pageNumber} exists`
      );
    }
  });

  function testSpread(assert) {
    const { spread, currentPage } = this.getProperties('spread', 'currentPage');
    for (
      var pageNumber = currentPage - spread;
      pageNumber <= currentPage + spread;
      pageNumber++
    ) {
      assert.ok(
        findAll(`.link.page-${pageNumber}`).length,
        `Page links for currentPage (${currentPage}) +/- spread of ${spread} (${pageNumber})`
      );
    }
  }

  function testItems(assert) {
    const { currentPage, size } = this.getProperties('currentPage', 'size');
    for (var item = 0; item < size; item++) {
      assert.equal(
        findAll('.item')[item].textContent,
        item + (currentPage - 1) * size,
        `Rendered items are in the current page, ${currentPage} (${
          item + (currentPage - 1) * size
        })`
      );
    }
  }
});
