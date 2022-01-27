import Controller from '@ember/controller';

export default class JobController extends Controller {
  get job() {
    return this.model;
  }
}
