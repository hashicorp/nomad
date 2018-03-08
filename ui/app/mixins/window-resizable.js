import Mixin from '@ember/object/mixin';
import { run } from '@ember/runloop';
import { assert } from '@ember/debug';
import $ from 'jquery';

export default Mixin.create({
  windowResizeHandler() {
    assert('windowResizeHandler needs to be overridden in the Component', false);
  },

  setupWindowResize: function() {
    run.scheduleOnce('afterRender', this, () => {
      this.set('_windowResizeHandler', this.get('windowResizeHandler').bind(this));
      $(window).on('resize', this.get('_windowResizeHandler'));
    });
  }.on('didInsertElement'),

  removeWindowResize: function() {
    $(window).off('resize', this.get('_windowResizeHandler'));
  }.on('willDestroyElement'),
});
