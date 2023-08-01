import Mixin from '@ember/object/mixin';

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  setupController(controller) {
    if (this.isForbidden) {
      this.set('isForbidden', undefined);
      controller.set('isForbidden', true);
    }
    this._super(...arguments);
  },

  resetController(controller) {
    controller.set('isForbidden', false);
    this._super(...arguments);
  },
});
