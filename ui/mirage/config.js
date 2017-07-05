import Ember from 'ember';

const { copy } = Ember;

export default function() {
  // These comments are here to help you get started. Feel free to delete them.

  /*
    Config (with defaults).

    Note: these only affect routes defined *after* them!
  */

  // this.urlPrefix = '';    // make this `http://localhost:8080`, for example, if your API is on a different server
  // this.namespace = '';    // make this `/api`, for example, if your API is namespaced
  this.timing = 400; // delay for each request, automatically set to 0 during testing

  /*
    Shorthand cheatsheet:

    this.get('/posts');
    this.post('/posts');
    this.get('/posts/:id');
    this.put('/posts/:id'); // or this.patch
    this.del('/posts/:id');

    http://www.ember-cli-mirage.com/docs/v0.3.x/shorthands/
  */
  this.namespace = 'v1';

  this.get('/jobs', function({ jobs }) {
    const json = this.serialize(jobs.all());
    return json.map(job => filterKeys(job, 'TaskGroups'));
  });

  this.get('/job/:id');

  this.get('/job/:id/summary', function({ jobSummaries }, { params }) {
    return this.serialize(jobSummaries.findBy({ jobId: params.id }));
  });
}

function filterKeys(object, ...keys) {
  const clone = copy(object, true);

  keys.forEach(key => {
    delete clone[key];
  });

  return clone;
}
