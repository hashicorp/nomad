import Component from '@ember/component';

// externalise and unit test
function activate(possible, $, pathname, cls) {
  if (pathname == '') {
    return;
  }
  const items = possible.filter(item => item.attributes['href'].value == pathname);
  if (items.length == 0) {
    return activate(
      possible,
      $,
      pathname
        .split('/')
        .slice(0, -1)
        .join('/'),
      cls
    );
  }
  return items.forEach(item =>
    $(item)
      .closest('li')
      .addClass(cls)
  );
}

function activateAnchors($, pathname = document.location.pathname, cls = 'is-active') {
  const possible = $('a[href]').get();
  return activate(possible, $, pathname, cls);
}
export default Component.extend({
  didRender() {
    this._super(...arguments);
    activateAnchors(this.$.bind(this));
  },
});
