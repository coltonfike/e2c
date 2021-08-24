
contract Hello {
  string h = "Hello World!";
  function hello() public view returns(string memory) {
    return h;
  }
}
