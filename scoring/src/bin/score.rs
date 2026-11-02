//! JSON stdin/stdout adapter; exits 0 on success, 2 on invalid input, and 1 on other failures.

mod score_command;

fn main() -> std::process::ExitCode {
    score_command::run()
}
