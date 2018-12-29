import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import RSVP from 'rsvp';

export default Route.extend({
  emberFreestyle: service(),

  beforeModel() {
    let emberFreestyle = this.get('emberFreestyle');

    return emberFreestyle.ensureHljs().then(() => {
      return RSVP.all([
        emberFreestyle.ensureHljsLanguage('handlebars'),
        emberFreestyle.ensureHljsLanguage('htmlbars'),
      ]);
    });
  },
});
