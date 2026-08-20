use std::process::Command;

#[test]
fn canonical_binary_reports_kura_name() {
    let output = Command::new(env!("CARGO_BIN_EXE_kura"))
        .arg("--version")
        .output()
        .expect("run kura --version");

    assert!(output.status.success());
    let stdout = String::from_utf8(output.stdout).expect("version output is utf-8");
    assert!(
        stdout.starts_with("kura "),
        "unexpected version output: {stdout}"
    );
}
