/**
 * 业务日期使用运行环境的本地日历日期。
 * 不使用 toISOString，避免中国时区凌晨被换算成前一天。
 */
export function businessToday() {
  const date = new Date();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}
