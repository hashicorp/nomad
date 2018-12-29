import { inject as service } from '@ember/service';
import FreestyleController from 'ember-freestyle/controllers/freestyle';

export default FreestyleController.extend({
  queryParams: {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  sortProperty: 'name',
  sortDescending: true,

  emberFreestyle: service(),
});
