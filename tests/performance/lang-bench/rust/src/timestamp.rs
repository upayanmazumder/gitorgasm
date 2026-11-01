/// Parses GitHub's fixed "YYYY-MM-DDTHH:MM:SSZ" layout like the Go benchmark.
pub fn parse_timestamp(s: &str) -> Result<i64, String> {
    let b = s.as_bytes();
    if b.len() != 20
        || b[4] != b'-'
        || b[7] != b'-'
        || b[10] != b'T'
        || b[13] != b':'
        || b[16] != b':'
        || b[19] != b'Z'
    {
        return Err(format!("bad timestamp {s:?}"));
    }
    let num = |a: usize, e: usize| -> Result<i64, String> {
        let mut v = 0i64;
        for &c in &b[a..e] {
            if !c.is_ascii_digit() {
                return Err(format!("bad timestamp {s:?}"));
            }
            v = v * 10 + (c - b'0') as i64;
        }
        Ok(v)
    };
    let (y, mo, d) = (num(0, 4)?, num(5, 7)?, num(8, 10)?);
    let (h, mi, se) = (num(11, 13)?, num(14, 16)?, num(17, 19)?);
    Ok(days_from_civil(y, mo, d) * 86400 + h * 3600 + mi * 60 + se)
}

/// Howard Hinnant's algorithm.
fn days_from_civil(mut y: i64, m: i64, d: i64) -> i64 {
    if m <= 2 {
        y -= 1;
    }
    let era = y.div_euclid(400);
    let yoe = y - era * 400;
    let mp = (m + 9) % 12;
    let doy = (153 * mp + 2) / 5 + d - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}
