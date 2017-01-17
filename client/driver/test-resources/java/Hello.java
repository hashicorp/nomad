public class Hello {
	public static void main(String[] args) {
		while (true) {
			System.out.println("Hello");
			try {
				Thread.sleep(1000); //1000 milliseconds is one second.
			} catch(InterruptedException ex) {
				Thread.currentThread().interrupt();
			}
		}
	}
}
