ulimit -n 524288
ulimit -l unlimited

case "$(hostname)" in
  *-login-0)
    ulimit -v 41943040 # 40gb, chosen bc vscode likes 32gb
    ;;
esac
