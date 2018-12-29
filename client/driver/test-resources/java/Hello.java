public class Hello {
	public static void main(String[] args) {
        System.out.println("Hello");
        int seconds = 5;
        if (args.length != 0) {
            seconds = Integer.parseInt(args[0]);
        }
        try {
            Thread.sleep(1000*seconds); //1000 milliseconds is one second.
        } catch(InterruptedException ex) {
            Thread.currentThread().interrupt();
        }
	}
}
