import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { reads } from '@ember/object/computed';

export default Component.extend({
  breadcrumbsService: service('breadcrumbs'),

  tagName: '',

  breadcrumbs: reads('breadcrumbsService.breadcrumbs'),
});
