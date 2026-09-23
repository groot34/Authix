export function isValidEmail(value: string): boolean {
  const email = value.trim();
  const at = email.indexOf('@');
  return at > 0 && at === email.lastIndexOf('@') && at < email.length - 1;
}