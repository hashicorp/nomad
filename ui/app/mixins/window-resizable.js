import Mixin from '@ember/object/mixin';
import { run } from '@ember/runloop';
import { assert } from '@ember/debug';
import { on } from '@ember/object/evented';
import $ from 'jquery';

export default Mixin.create({
  windowResizeHandler() {
    assert('windowResizeHandler needs to be overridden in the Component', false);
  },

  setupWindowResize: on('didInsertElement', function() {
    run.scheduleOnce('afterRender', this, () => {
      this.set('_windowResizeHandler', this.windowResizeHandler.bind(this));
      $(window).on('resize', this._windowResizeHandler);
    });
  }),

  removeWindowResize: on('willDestroyElement', function() {
    $(window).off('resize', this._windowResizeHandler);
  }),
});
