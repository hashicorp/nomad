import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class RunController extends Controller {
  @service router;
  onSubmit(id, namespace) {
    this.router.transitionTo('jobs.job', `${id}@${namespace || 'default'}`);
  }

  @action uploadJobSpec(event) {
    const reader = new FileReader();
    reader.onload = () => {
      this.model._newDefinition = reader.result;
    };

    const [file] = event.target.files;
    reader.readAsText(file);
  }
}
