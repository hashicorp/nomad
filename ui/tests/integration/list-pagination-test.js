import { findAll, find } from 'ember-native-dom-helpers';
import { test, skip, moduleForComponent } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('list-pagination', 'Integration | Component | list pagination', {
  integration: true,
});

const defaults = {
  source: [],
  size: 25,
  page: 1,
  spread: 2,
};

const list100 = Array(100)
  .fill(null)
  .map((_, i) => i);

test('the source property', function(assert) {
  this.set('source', list100);
  this.render(hbs`
    {{#list-pagination source=source as |p|}}
      <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
      {{#p.first}}<span class="first">first</span>{{/p.first}}
      {{#p.prev}}<span class="prev">prev</span>{{/p.prev}}
      {{#each p.pageLinks as |link|}}
        <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
      {{/each}}
      {{#p.next}}<span class="next">next</span>{{/p.next}}
      {{#p.last}}<span class="last">last</span>{{/p.last}}

      {{#each p.list as |item|}}
        <div class="item">{{item}}</div>
      {{/each}}
    {{/list-pagination}}
  `);

  assert.ok(!findAll('.first').length, 'On the first page, there is no first link');
  assert.ok(!findAll('.prev').length, 'On the first page, there is no prev link');

  assert.equal(
    findAll('.link').length,
    defaults.spread + 1,
    'Pages links spread to the right by the spread amount'
  );

  for (var pageNumber = 1; pageNumber <= defaults.spread + 1; pageNumber++) {
    assert.ok(findAll(`.link.page-${pageNumber}`).length, `Page link includes ${pageNumber}`);
  }

  assert.ok(findAll('.next').length, 'While not on the last page, there is a next link');
  assert.ok(findAll('.last').length, 'While not on the last page, there is a last link');

  assert.ok(
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

test('the size property', function(assert) {
  this.setProperties({
    size: 5,
    source: list100,
  });
  this.render(hbs`
    {{#list-pagination source=source size=size as |p|}}
      <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
    {{/list-pagination}}
  `);

  const totalPages = Math.ceil(this.get('source').length / this.get('size'));
  assert.equal(find('.page-info').textContent, `1 of ${totalPages}`, `${totalPages} total pages`);
});

test('the spread property', function(assert) {
  this.setProperties({
    source: list100,
    spread: 1,
    size: 10,
    currentPage: 5,
  });

  this.render(hbs`
    {{#list-pagination source=source spread=spread size=size page=currentPage as |p|}}
      {{#each p.pageLinks as |link|}}
        <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
      {{/each}}
    {{/list-pagination}}
  `);

  testSpread.call(this, assert);
  this.set('spread', 4);
  testSpread.call(this, assert);
});

test('page property', function(assert) {
  this.setProperties({
    source: list100,
    size: 5,
    currentPage: 5,
  });

  this.render(hbs`
    {{#list-pagination source=source size=size page=currentPage as |p|}}
      {{#each p.list as |item|}}
        <div class="item">{{item}}</div>
      {{/each}}
    {{/list-pagination}}
  `);

  testItems.call(this, assert);
  this.set('currentPage', 2);
  testItems.call(this, assert);
});

// Ember doesn't support query params (or controllers or routes) in integration tests,
// so links can only be tested in acceptance tests.
// Leaving this test here for posterity.
skip('pagination links link with query params', function() {});

test('there are no pagination links when source is less than page size', function(assert) {
  this.set('source', list100.slice(0, 10));
  this.render(hbs`
    {{#list-pagination source=source as |p|}}
      <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
      {{#p.first}}<span class="first">first</span>{{/p.first}}
      {{#p.prev}}<span class="prev">prev</span>{{/p.prev}}
      {{#each p.pageLinks as |link|}}
        <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
      {{/each}}
      {{#p.next}}<span class="next">next</span>{{/p.next}}
      {{#p.last}}<span class="last">last</span>{{/p.last}}

      {{#each p.list as |item|}}
        <div class="item">{{item}}</div>
      {{/each}}
    {{/list-pagination}}
  `);

  assert.ok(!findAll('.first').length, 'No first link');
  assert.ok(!findAll('.prev').length, 'No prev link');
  assert.ok(!findAll('.next').length, 'No next link');
  assert.ok(!findAll('.last').length, 'No last link');

  assert.equal(find('.page-info').textContent, '1 of 1', 'Only one page');
  assert.equal(
    findAll('.item').length,
    this.get('source.length'),
    'Number of items equals length of source'
  );
});

// when there are no items in source
test('when there are no items in source', function(assert) {
  this.set('source', []);
  this.render(hbs`
    {{#list-pagination source=source as |p|}}
      <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
      {{#p.first}}<span class="first">first</span>{{/p.first}}
      {{#p.prev}}<span class="prev">prev</span>{{/p.prev}}
      {{#each p.pageLinks as |link|}}
        <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
      {{/each}}
      {{#p.next}}<span class="next">next</span>{{/p.next}}
      {{#p.last}}<span class="last">last</span>{{/p.last}}

      {{#each p.list as |item|}}
        <div class="item">{{item}}</div>
      {{/each}}
    {{else}}
      <div class="empty-state">Empty State</div>
    {{/list-pagination}}
  `);

  assert.ok(
    !findAll('.page-info, .first, .prev, .link, .next, .last, .item').length,
    'Nothing in the yield renders'
  );
  assert.ok(findAll('.empty-state').length, 'Empty state is rendered');
});

// when there is less pages than the total spread amount
test('when there is less pages than the total spread amount', function(assert) {
  this.setProperties({
    source: list100,
    spread: 4,
    size: 20,
    page: 3,
  });

  const totalPages = Math.ceil(this.get('source.length') / this.get('size'));

  this.render(hbs`
    {{#list-pagination source=source page=page spread=spread size=size as |p|}}
      <span class="page-info">{{p.currentPage}} of {{p.totalPages}}</span>
      {{#p.first}}<span class="first">first</span>{{/p.first}}
      {{#p.prev}}<span class="prev">prev</span>{{/p.prev}}
      {{#each p.pageLinks as |link|}}
        <span class="link page-{{link.pageNumber}}">{{link.pageNumber}}</span>
      {{/each}}
      {{#p.next}}<span class="next">next</span>{{/p.next}}
      {{#p.last}}<span class="last">last</span>{{/p.last}}
    {{/list-pagination}}
  `);

  assert.ok(findAll('.first').length, 'First page still exists');
  assert.ok(findAll('.prev').length, 'Prev page still exists');
  assert.ok(findAll('.next').length, 'Next page still exists');
  assert.ok(findAll('.last').length, 'Last page still exists');
  assert.equal(findAll('.link').length, totalPages, 'Every page gets a page link');
  for (var pageNumber = 1; pageNumber < totalPages; pageNumber++) {
    assert.ok(findAll(`.link.page-${pageNumber}`).length, `Page link for ${pageNumber} exists`);
  }
});

function testSpread(assert) {
  const { spread, currentPage } = this.getProperties('spread', 'currentPage');
  for (var pageNumber = currentPage - spread; pageNumber <= currentPage + spread; pageNumber++) {
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
      `Rendered items are in the current page, ${currentPage} (${item + (currentPage - 1) * size})`
    );
  }
}
