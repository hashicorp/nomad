import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';

export default class JobsRunTemplatesController extends Controller {
  @tracked selectedTemplate = null;

  columns = ['name', 'namespace', 'description'].map((column) => {
    return {
      key: column,
      label: `${column.charAt(0).toUpperCase()}${column.substring(1)}`,
    };
  });

  formatTemplateLabel(path) {
    return path.split('nomad/job-templates/')[1];
  }
}
