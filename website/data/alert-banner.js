export const ALERT_BANNER_ACTIVE = true
// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  tag: 'Survey',
  url: 'https://docs.google.com/forms/d/e/1FAIpQLSeyDEyQXzkijZnkXjj8qVb_5IydajRkFnOrPjDNoysFs-6jDQ/viewform',
  text: 'Using Nomad for edge workloads? We want to hear about it!',
  linkText: 'Fill out our user survey',
  // Set the expirationDate prop with a datetime string (e.g. '2020-01-31T12:00:00-07:00')
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: '2022-02-08T23:00:00-07:00',
}
