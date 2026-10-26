//! Minimal axum + tokio service used to measure idle RSS.
//! Default tokio multi-thread runtime (one worker per CPU), like Go's
//! default GOMAXPROCS.

use axum::{Router, routing::get};

#[tokio::main]
async fn main() {
    let addr = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "127.0.0.1:18081".into());
    let app = Router::new().route("/", get(|| async { "hello" }));
    let listener = tokio::net::TcpListener::bind(&addr).await.expect("bind");
    eprintln!("listening on {addr}");
    axum::serve(listener, app).await.expect("serve");
}
