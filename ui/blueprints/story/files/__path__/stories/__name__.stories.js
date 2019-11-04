import hbs from 'htmlbars-inline-precompile';

export default {
  title: '<%= classifiedModuleName %>',
};

export const <%= classifiedModuleName %> = () => {
  return {
    template: hbs`
      <h5 class="title is-5"><%= header %></h5>
      <<%= classifiedModuleName %>/>
    `,
    context: {},
  }
};
